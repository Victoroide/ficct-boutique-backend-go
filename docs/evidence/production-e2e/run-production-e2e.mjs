// Production E2E evidence runner — hits the real deployed services and writes
// JSON evidence + a summary into a timestamped folder. HTTP-only, no secrets:
// the valid invoice flow is triggered by confirmSale (Go signs the webhook with
// its own secret); the negative test posts an invalid signature.
//
//   node docs/evidence/production-e2e/run-production-e2e.mjs
import crypto from 'node:crypto';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const GO = 'https://ficct-boutique-backend-go-production.up.railway.app';
const N8N = 'https://n8n-production-6287.up.railway.app';
const GOTENBERG = 'https://gotenberg-production-7558.up.railway.app';
const MS3 = 'https://bptu80mcbk.execute-api.us-east-1.amazonaws.com';
const MS3_DOMAIN = 'https://docs-api-boutique.ficct.com';
const MS2 = 'https://ficct-ai-1093089304525.us-central1.run.app';

const here = dirname(fileURLToPath(import.meta.url));
const stamp = new Date().toISOString().replace(/[:T]/g, '').slice(0, 15).replace(/(\d{8})(\d{6})/, '$1-$2');
const out = join(here, stamp);
mkdirSync(out, { recursive: true });
const save = (name, obj) => writeFileSync(join(out, name), JSON.stringify(obj, null, 2) + '\n');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function gql(query, variables = {}, token = '') {
  const res = await fetch(GO + '/graphql', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
    body: JSON.stringify({ query, variables }),
  });
  const body = await res.json();
  if (!res.ok || body.errors?.length) throw new Error('GraphQL: ' + JSON.stringify(body.errors || body));
  return body.data;
}
async function code(url, opts) { try { const r = await fetch(url, opts); return r.status; } catch (e) { return 'ERR ' + e.message; } }

const results = { startedAt: new Date().toISOString(), out };

// ---------- Phase 1: endpoints ----------
const endpoints = {
  go_health: await code(GO + '/health'),
  n8n_health: await code(N8N + '/healthz'),
  gotenberg_health: await code(GOTENBERG + '/health'),
  ms3_health: await code(MS3 + '/health'),
  ms3_domain_health: await code(MS3_DOMAIN + '/health'),
  ms2_health_public: await code(MS2 + '/api/v1/health/'),
};
save('endpoints.json', endpoints);
console.log('endpoints', JSON.stringify(endpoints));

// ---------- MS1 Go core ----------
const ms1 = { steps: {} };
try {
  const staff = (await gql('mutation L($i:LoginInput!){login(input:$i){accessToken user{id email role}}}', { i: { email: 'staff@ficct.local', password: 'Staff123!' } })).login;
  const customer = (await gql('mutation L($i:LoginInput!){login(input:$i){accessToken user{id email role}}}', { i: { email: 'cliente@ficct.local', password: 'Cliente123!' } })).login;
  ms1.steps.login = { staff: staff.user, customer: customer.user };
  const sToken = staff.accessToken, cToken = customer.accessToken;

  const cat = await gql('query{ products(limit:5){ sku name variants{ id size color stock{ quantity reorderLevel } } } branches{ id code name } }', {}, sToken);
  ms1.steps.products = cat.products;
  ms1.steps.branches = cat.branches;

  const inv = await gql('query{ inventoryEntries(limit:5){ total entries{ variantId quantity reorderLevel branch{ code } } } }', {}, sToken);
  ms1.steps.inventory = inv.inventoryEntries;

  const bi = await gql('query{ monthlySales(months:6){ month totalSales saleCount } popularProducts(limit:5){ productName unitsSold revenue } dashboardSummary{ __typename } }', {}, sToken);
  ms1.steps.reports = bi;

  const product = cat.products.find((p) => p.variants.length > 0);
  const branch = cat.branches[0];
  const sale = (await gql('mutation C($i:CreateSaleInput!){ createSale(input:$i){ id status total } }', { i: { customerId: customer.user.id, branchId: branch.id, items: [{ variantId: product.variants[0].id, quantity: 1 }] } }, cToken)).createSale;
  const order = (await gql('mutation C($id:UUID!){ confirmSale(saleId:$id){ id code status } }', { id: sale.id }, cToken)).confirmSale;
  ms1.steps.sale = { saleId: sale.id, total: sale.total, orderCode: order.code, status: order.status };
  ms1.ok = true;
  results.orderCode = order.code; results.staffToken = sToken;
  console.log('MS1 ok; orderCode', order.code);
} catch (e) { ms1.ok = false; ms1.error = e.message; console.log('MS1 FAIL', e.message); }
save('ms1-go-results.json', ms1);

// ---------- MS3 direct (AWS) ----------
const ms3 = { base: MS3, steps: {} };
try {
  const token = results.staffToken;
  const pdf = Buffer.from('%PDF-1.4\nFICCT production E2E evidence\n%%EOF\n');
  const sha = crypto.createHash('sha256').update(pdf).digest('hex');
  const ur = await (await fetch(MS3 + '/api/v1/documents/upload-request', { method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' }, body: JSON.stringify({ title: 'Prod E2E', description: 'evidence', category: 'pdf', mimeType: 'application/pdf', sizeBytes: pdf.length, metadata: { e2e: true } }) })).json();
  ms3.steps.uploadRequest = { docId: ur.document?.id, urlHost: ur.upload?.url ? new URL(ur.upload.url).host : null };
  const put = await fetch(ur.upload.url, { method: 'PUT', headers: ur.upload.headers || { 'Content-Type': 'application/pdf' }, body: pdf });
  ms3.steps.s3Put = put.status;
  const cf = await (await fetch(MS3 + '/api/v1/documents/' + ur.document.id + '/confirm', { method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' }, body: JSON.stringify({ sha256: sha }) })).json();
  ms3.steps.confirm = cf.document?.status;
  const vf = await (await fetch(MS3 + '/api/v1/documents/' + ur.document.id + '/verify', { headers: { Authorization: 'Bearer ' + token } })).json();
  ms3.steps.verify = { intact: vf.intact, chainIntact: vf.chainIntact };
  ms3.steps.s3Key = ur.document?.storage_key;
  ms3.ok = put.status === 200 && cf.document?.status === 'active' && vf.intact === true && vf.chainIntact === true;
  console.log('MS3 ok', ms3.ok);
} catch (e) { ms3.ok = false; ms3.error = e.message; console.log('MS3 FAIL', e.message); }
save('ms3-results.json', ms3);

// ---------- Automation: invoice doc for the confirmed sale + invalid HMAC ----------
const automation = { steps: {} };
try {
  const token = results.staffToken;
  let doc = null;
  for (let i = 0; i < 20 && results.orderCode; i++) {
    const list = await (await fetch(MS3 + '/api/v1/documents?category=pdf&status=active&limit=100', { headers: { Authorization: 'Bearer ' + token } })).json();
    doc = (list.documents || []).find((d) => d.metadata?.orderCode === results.orderCode);
    if (doc) break;
    await sleep(3000);
  }
  if (doc) {
    const vf = await (await fetch(MS3 + '/api/v1/documents/' + doc.id + '/verify', { headers: { Authorization: 'Bearer ' + token } })).json();
    automation.steps.invoiceDocument = { id: doc.id, status: doc.status, sha256: doc.sha256, storage_key: doc.storage_key, intact: vf.intact, chainIntact: vf.chainIntact };
  } else {
    automation.steps.invoiceDocument = { found: false, note: 'invoice doc not found for orderCode within timeout' };
  }
  // invalid HMAC
  const badCode = 'ORD-BAD-' + Date.now();
  const badPayload = { event: 'sale.confirmed', sale_id: crypto.randomUUID(), order_id: crypto.randomUUID(), order_code: badCode, items: [{ variant_id: 'x', quantity: 1, unit_price: 1, line_total: 1 }], total: 1, currency: 'BOB', branch_id: crypto.randomUUID(), confirmed_at: new Date().toISOString(), customer: { email: 'bad@ficct.local', name: 'Bad' } };
  const badResp = await fetch(N8N + '/webhook/ficct-invoice', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-FICCT-Signature': 'sha256=invalid', 'X-FICCT-Event': 'sale.confirmed' }, body: JSON.stringify(badPayload) });
  await sleep(6000);
  const list2 = await (await fetch(MS3 + '/api/v1/documents?category=pdf&status=active&limit=100', { headers: { Authorization: 'Bearer ' + token } })).json();
  const badDoc = (list2.documents || []).find((d) => d.metadata?.orderCode === badCode);
  automation.steps.invalidHmac = { webhookHttp: badResp.status, sideEffectDocument: badDoc ? badDoc.id : null, noSideEffect: !badDoc };
  automation.ok = !!doc && !badDoc;
  console.log('automation ok', automation.ok);
} catch (e) { automation.ok = false; automation.error = e.message; console.log('automation FAIL', e.message); }
save('automation-results.json', automation);

// ---------- MS2 Django AI ----------
const ms2 = { base: MS2, steps: {} };
try {
  ms2.steps.publicHealth = await (await fetch(MS2 + '/api/v1/health/')).json();
  const token = results.staffToken;
  const emb = await fetch(MS2 + '/api/v1/ai/catalog/embeddings/', { headers: { Authorization: 'Bearer ' + token } });
  ms2.steps.embeddingsRead = { http: emb.status, body: await emb.json().catch(() => null) };
  const seg = await fetch(MS2 + '/api/v1/clustering/segments/', { headers: { Authorization: 'Bearer ' + token } });
  ms2.steps.segmentsRead = { http: seg.status };
  ms2.steps.endpoints = ['/api/v1/ai/similarity/search/', '/api/v1/ai/catalog/sync/', '/api/v1/ai/catalog/embeddings/', '/api/v1/forecasting/run/', '/api/v1/forecasting/latest/<scope>/', '/api/v1/clustering/run/', '/api/v1/clustering/segments/'];
  ms2.ok = ms2.steps.embeddingsRead.http < 500;
  console.log('MS2 ok', ms2.ok);
} catch (e) { ms2.ok = false; ms2.error = e.message; console.log('MS2 FAIL', e.message); }
save('ms2-results.json', ms2);

delete results.staffToken; // never persist the bearer token to evidence
results.finishedAt = new Date().toISOString();
results.pass = { ms1: ms1.ok, ms3: ms3.ok, automation: automation.ok, ms2: ms2.ok, endpoints };
save('run-results.json', results);
console.log('RUN_FOLDER=' + out);
