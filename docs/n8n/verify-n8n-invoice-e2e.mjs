import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const { parse: parseFlatted } = require('/app/node_modules/flatted');

const GO_GRAPHQL_URL = process.env.GO_GRAPHQL_URL || 'http://go-core:8080/graphql';
const EXPRESS_DOCS_URL = process.env.EXPRESS_DOCS_URL || 'http://express-docs:8081';
const N8N_WEBHOOK_URL = process.env.N8N_WEBHOOK_URL || 'http://n8n:5678/webhook/ficct-invoice';
const MAILPIT_API_URL = process.env.MAILPIT_API_URL || 'http://mailpit:8025';
const N8N_SQLITE_PATH = process.env.N8N_SQLITE_PATH || '/n8n-data/.n8n/database.sqlite';

function requireEnv(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

const E2E_STAFF_EMAIL = requireEnv('E2E_STAFF_EMAIL');
const E2E_STAFF_PASSWORD = requireEnv('E2E_STAFF_PASSWORD');
const E2E_CUSTOMER_EMAIL = requireEnv('E2E_CUSTOMER_EMAIL');
const E2E_CUSTOMER_PASSWORD = requireEnv('E2E_CUSTOMER_PASSWORD');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function retry(label, fn, { attempts = 60, delayMs = 2000 } = {}) {
  let lastError;
  for (let i = 1; i <= attempts; i += 1) {
    try {
      const value = await fn();
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await sleep(delayMs);
  }
  throw new Error(`${label} did not become ready${lastError ? `: ${lastError.message}` : ''}`);
}

async function waitForHttp(label, url) {
  await retry(label, async () => {
    const res = await fetch(url);
    return res.status < 500;
  });
  console.log(`[ok] ${label} is reachable`);
}

async function gql(query, variables = {}, token = '') {
  const res = await fetch(GO_GRAPHQL_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ query, variables }),
  });
  const body = await res.json();
  if (!res.ok || body.errors?.length) {
    throw new Error(`GraphQL failed: ${JSON.stringify(body.errors || body)}`);
  }
  return body.data;
}

async function login(email, password) {
  const data = await gql(
    `mutation Login($input: LoginInput!) {
      login(input: $input) {
        accessToken
        user { id email role }
      }
    }`,
    { input: { email, password } },
  );
  return data.login;
}

function customerIdForEmail(email) {
  try {
    const sql = `SELECT c.id FROM customers c JOIN users u ON u.id = c.user_id WHERE lower(u.email) = lower('${email.replace(/'/g, "''")}') LIMIT 1`;
    const out = execFileSync('psql', ['-At', '-c', sql], {
      encoding: 'utf8',
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim();
    return out || null;
  } catch {
    return null;
  }
}

function sqliteJson(sql) {
  const out = execFileSync('sqlite3', ['-readonly', '-json', N8N_SQLITE_PATH, sql], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  }).trim();
  return out ? JSON.parse(out) : [];
}

function workflowExecutions(limit = 20) {
  return sqliteJson(`
    SELECT e.id, e.status, e.finished, e.startedAt, e.stoppedAt, d.data
    FROM execution_entity e
    LEFT JOIN execution_data d ON d.executionId = e.id
    WHERE e.workflowId = 'ficct-invoice-workflow'
    ORDER BY e.id DESC
    LIMIT ${Number(limit)}
  `);
}

function executionSummary(row) {
  if (!row?.data) return { nodes: [], errors: [], text: '' };
  const text = String(row.data);
  try {
    const parsed = parseFlatted(text);
    const runData = parsed?.resultData?.runData || {};
    const errors = [];
    for (const [nodeName, runs] of Object.entries(runData)) {
      for (const run of runs || []) {
        if (run?.error) {
          errors.push({ node: nodeName, message: run.error.message || String(run.error) });
        }
      }
    }
    const resultError = parsed?.resultData?.error;
    if (resultError) {
      errors.push({
        node: resultError.node?.name || resultError.node || parsed?.resultData?.lastNodeExecuted || 'unknown',
        message: resultError.message || String(resultError),
      });
    }
    return { nodes: Object.keys(runData), errors, text };
  } catch (error) {
    return { nodes: [], errors: [{ node: 'execution_data', message: error.message }], text };
  }
}

async function waitForN8nSuccess(orderCode) {
  return retry('n8n successful invoice execution', () => {
    const executions = workflowExecutions(30);
    const match = executions.find((row) =>
      row.status === 'success' &&
      row.data &&
      String(row.data).includes(orderCode));
    if (!match) return null;
    const summary = executionSummary(match);
    const requiredNodes = [
      'Validate HMAC and Payload',
      'Generate PDF with Gotenberg',
      'Compute PDF SHA-256',
      'MS3 Confirm Upload',
      'MS3 Verify Hash Ledger',
      'Send Invoice Email',
    ];
    const missing = requiredNodes.filter((node) => !summary.nodes.includes(node));
    if (missing.length) {
      throw new Error(`n8n success execution ${match.id} missing nodes: ${missing.join(', ')}`);
    }
    return { ...match, summary };
  }, { attempts: 30, delayMs: 1000 });
}

async function waitForInvalidSignatureExecution() {
  return retry('n8n invalid-signature execution', () => {
    const executions = workflowExecutions(30);
    const match = executions.find((row) => {
      if (row.status !== 'error' || !row.data) return false;
      const summary = executionSummary(row);
      return summary.errors.some((error) => error.message.includes('Invalid FICCT webhook signature'));
    });
    if (!match) return null;
    return { ...match, summary: executionSummary(match) };
  }, { attempts: 30, delayMs: 1000 });
}

async function expressJson(path, token) {
  const res = await fetch(`${EXPRESS_DOCS_URL}${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const body = await res.json();
  if (!res.ok) {
    throw new Error(`Express ${path} failed: ${JSON.stringify(body)}`);
  }
  return body;
}

async function clearMailpit() {
  try {
    await fetch(`${MAILPIT_API_URL}/api/v1/messages`, { method: 'DELETE' });
  } catch {
    // Older Mailpit builds may not expose bulk delete; polling below filters by subject.
  }
}

async function mailpitMessages() {
  const res = await fetch(`${MAILPIT_API_URL}/api/v1/messages`);
  if (!res.ok) throw new Error(`Mailpit messages failed with ${res.status}`);
  const body = await res.json();
  return body.messages || body.Messages || [];
}

function messageSubject(message) {
  return message.Subject || message.subject || '';
}

function messageId(message) {
  return message.ID || message.Id || message.id;
}

function messageRecipients(message) {
  const raw = message.To || message.to || [];
  return JSON.stringify(raw).toLowerCase();
}

async function messageDetail(id) {
  const res = await fetch(`${MAILPIT_API_URL}/api/v1/message/${id}`);
  if (!res.ok) return null;
  return res.json();
}

function attachmentCount(message, detail) {
  const direct = message.Attachments ?? message.attachments;
  if (typeof direct === 'number') return direct;
  if (Array.isArray(direct)) return direct.length;
  const detailed = detail?.Attachments ?? detail?.attachments;
  if (typeof detailed === 'number') return detailed;
  if (Array.isArray(detailed)) return detailed.length;
  return 0;
}

async function findInvoiceDocument(orderCode, staffToken) {
  const body = await expressJson('/api/v1/documents?category=pdf&status=active&limit=100', staffToken);
  return (body.documents || []).find((doc) => doc.metadata?.orderCode === orderCode);
}

async function main() {
  await waitForHttp('Go GraphQL', GO_GRAPHQL_URL.replace('/graphql', '/health'));
  await waitForHttp('Express docs', `${EXPRESS_DOCS_URL}/health`);
  await waitForHttp('n8n', N8N_WEBHOOK_URL.replace('/webhook/ficct-invoice', '/healthz'));
  await waitForHttp('Mailpit', `${MAILPIT_API_URL}/api/v1/messages`);

  await clearMailpit();

  const customerLogin = await login(E2E_CUSTOMER_EMAIL, E2E_CUSTOMER_PASSWORD);
  const staffLogin = await login(E2E_STAFF_EMAIL, E2E_STAFF_PASSWORD);
  const customerToken = customerLogin.accessToken;
  const staffToken = staffLogin.accessToken;
  const customerId = customerIdForEmail(customerLogin.user.email) || customerLogin.user.id;

  const seed = await gql(
    `query SeedData {
      branches { id code }
      products(limit: 10) {
        sku
        variants { id }
      }
    }`,
    {},
    customerToken,
  );

  const branch = seed.branches[0];
  const product = seed.products.find((p) => p.variants.length > 0);
  if (!branch || !product) throw new Error('Seed branch/product data is missing');
  const variantId = product.variants[0].id;

  const saleData = await gql(
    `mutation CreateSale($input: CreateSaleInput!) {
      createSale(input: $input) {
        id
        status
        total
      }
    }`,
    {
      input: {
        customerId,
        branchId: branch.id,
        items: [{ variantId, quantity: 1 }],
      },
    },
    customerToken,
  );

  const saleId = saleData.createSale.id;
  const orderData = await gql(
    `mutation ConfirmSale($saleId: UUID!) {
      confirmSale(saleId: $saleId) {
        id
        code
        status
      }
    }`,
    { saleId },
    customerToken,
  );
  const orderCode = orderData.confirmSale.code;
  console.log(`[ok] confirmed sale ${saleId} as ${orderCode}`);

  const message = await retry('Mailpit invoice email', async () => {
    const messages = await mailpitMessages();
    return messages.find((msg) =>
      messageSubject(msg).includes(`Factura ${orderCode} - FICCT Boutique`) &&
      messageRecipients(msg).includes(E2E_CUSTOMER_EMAIL.toLowerCase()));
  }, { attempts: 75, delayMs: 2000 });

  const detail = await messageDetail(messageId(message));
  if (attachmentCount(message, detail) < 1) {
    throw new Error('Invoice email arrived without a PDF attachment');
  }
  const detailText = JSON.stringify(detail || message);
  if (!detailText.includes(orderCode)) {
    throw new Error('Invoice email detail does not contain the order code');
  }
  console.log(`[ok] Mailpit received invoice email for ${orderCode} with attachment`);

  const successExecution = await waitForN8nSuccess(orderCode);
  console.log(`[ok] n8n execution ${successExecution.id} succeeded through ${successExecution.summary.nodes.length} nodes`);

  const document = await retry('MS3 active invoice document', () => findInvoiceDocument(orderCode, staffToken), {
    attempts: 30,
    delayMs: 1000,
  });
  if (document.status !== 'active' || document.mime_type !== 'application/pdf' || !document.sha256) {
    throw new Error(`Invoice document is not active PDF with SHA-256: ${JSON.stringify(document)}`);
  }

  const verify = await expressJson(`/api/v1/documents/${document.id}/verify`, staffToken);
  if (verify.intact !== true || verify.chainIntact !== true) {
    throw new Error(`Hash ledger verification failed: ${JSON.stringify(verify)}`);
  }
  console.log(`[ok] MS3 stored active PDF ${document.id}; hash ledger verifies`);

  const badOrderCode = `ORD-BAD-SIGNATURE-${Date.now()}`;
  const badPayload = {
    event: 'sale.confirmed',
    sale_id: crypto.randomUUID(),
    order_id: crypto.randomUUID(),
    order_code: badOrderCode,
    items: [{ VariantID: variantId, Quantity: 1, UnitPrice: 1, LineTotal: 1 }],
    total: 1,
    currency: 'BOB',
    branch_id: branch.id,
    confirmed_at: new Date().toISOString(),
    customer: { email: 'bad-signature@ficct.local', name: 'Firma Invalida' },
  };
  await fetch(N8N_WEBHOOK_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-FICCT-Event': 'sale.confirmed',
      'X-FICCT-Signature': 'sha256=invalid',
      'X-FICCT-Event-Id': crypto.randomUUID(),
    },
    body: JSON.stringify(badPayload),
  });

  await sleep(5000);
  const invalidExecution = await waitForInvalidSignatureExecution();
  console.log(`[ok] n8n execution ${invalidExecution.id} failed on invalid HMAC as expected`);
  const badMessages = (await mailpitMessages()).filter((msg) => messageSubject(msg).includes(badOrderCode));
  const badDocument = await findInvoiceDocument(badOrderCode, staffToken);
  if (badMessages.length > 0 || badDocument) {
    throw new Error('Invalid-signature webhook produced side effects');
  }
  console.log('[ok] invalid signature was rejected before PDF/MS3/email side effects');

  console.log('[done] sale -> n8n -> Gotenberg PDF -> MS3/MinIO -> hash ledger -> Mailpit email verified');
}

main().catch((error) => {
  console.error(`[fail] ${error.stack || error.message}`);
  process.exit(1);
});
