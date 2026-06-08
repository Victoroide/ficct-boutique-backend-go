import { writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

const validateHmacCode = String.raw`
const crypto = require('crypto');

function env(name, fallback = '') {
  if (typeof $env !== 'undefined' && $env?.[name]) return $env[name];
  if (typeof process !== 'undefined' && process.env?.[name]) return process.env[name];
  return fallback;
}

const secret = env('WEBHOOK_HMAC_SECRET') || env('FICCT_WEBHOOK_HMAC_SECRET');
if (!secret) {
  throw new Error('WEBHOOK_HMAC_SECRET is required for FICCT invoice webhook validation');
}

const input = $input.first();
const headers = input.json.headers || {};
const signatureHeader = headers['x-ficct-signature'] || headers['X-FICCT-Signature'];
if (!signatureHeader) {
  throw new Error('Missing x-ficct-signature header');
}

if (!input.binary?.data) {
  throw new Error('Webhook raw body is missing; enable Raw Body on the Webhook node');
}

const rawBody = await this.helpers.getBinaryDataBuffer(0, 'data');
const expected = 'sha256=' + crypto
  .createHmac('sha256', secret)
  .update(rawBody)
  .digest('hex');

const actualBuffer = Buffer.from(String(signatureHeader), 'utf8');
const expectedBuffer = Buffer.from(expected, 'utf8');
if (actualBuffer.length !== expectedBuffer.length || !crypto.timingSafeEqual(actualBuffer, expectedBuffer)) {
  throw new Error('Invalid FICCT webhook signature');
}

let payload;
try {
  payload = JSON.parse(rawBody.toString('utf8'));
} catch (error) {
  throw new Error('Webhook body is not valid JSON: ' + error.message);
}

const required = [
  ['event', payload.event],
  ['sale_id', payload.sale_id],
  ['order_id', payload.order_id],
  ['order_code', payload.order_code],
  ['items', payload.items],
  ['total', payload.total],
  ['currency', payload.currency],
  ['branch_id', payload.branch_id],
  ['confirmed_at', payload.confirmed_at],
  ['customer.email', payload.customer?.email],
  ['customer.name', payload.customer?.name],
];

const missing = required
  .filter(([name, value]) => value === undefined || value === null || value === '' || (name === 'items' && (!Array.isArray(value) || value.length === 0)))
  .map(([name]) => name);

if (payload.event !== 'sale.confirmed') {
  throw new Error('Unsupported event: ' + payload.event);
}

if (missing.length) {
  throw new Error('Missing required invoice payload field(s): ' + missing.join(', '));
}

return [{
  json: {
    ...payload,
    webhook_event_id: headers['x-ficct-event-id'] || headers['X-FICCT-Event-Id'] || null,
    raw_body_sha256: crypto.createHash('sha256').update(rawBody).digest('hex'),
  },
}];
`.trim();

const buildHtmlCode = String.raw`
function esc(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function money(value) {
  const number = Number(value || 0);
  return number.toLocaleString('es-BO', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function itemField(item, snake, pascal) {
  return item?.[snake] ?? item?.[pascal] ?? '';
}

const payload = $input.first().json;
const orderCode = payload.order_code;
const invoiceFileBase = 'factura-' + String(orderCode).replace(/[^A-Za-z0-9_-]/g, '-').toLowerCase();
const invoiceFilename = invoiceFileBase + '.pdf';
const issuedAt = new Date(payload.confirmed_at).toLocaleString('es-BO', {
  timeZone: 'America/La_Paz',
  dateStyle: 'medium',
  timeStyle: 'short',
});

const rows = payload.items.map((item, index) => {
  const variantId = itemField(item, 'variant_id', 'VariantID');
  const quantity = itemField(item, 'quantity', 'Quantity');
  const unitPrice = itemField(item, 'unit_price', 'UnitPrice');
  const lineTotal = itemField(item, 'line_total', 'LineTotal');
  return '<tr>' +
    '<td>' + (index + 1) + '</td>' +
    '<td><strong>Variante</strong><br><span>' + esc(variantId) + '</span></td>' +
    '<td class="num">' + esc(quantity) + '</td>' +
    '<td class="num">' + money(unitPrice) + '</td>' +
    '<td class="num">' + money(lineTotal) + '</td>' +
  '</tr>';
}).join('');

const html = [
  '<!doctype html>',
  '<html lang="es">',
  '<head>',
  '  <meta charset="utf-8">',
  '  <title>Factura ' + esc(orderCode) + ' - FICCT Boutique</title>',
  '  <style>',
  '    @page { size: A4; margin: 22mm 18mm; }',
  '    * { box-sizing: border-box; }',
  '    body { color: #1f2933; font-family: Arial, Helvetica, sans-serif; font-size: 13px; line-height: 1.45; margin: 0; }',
  '    .top { align-items: flex-start; border-bottom: 2px solid #22304a; display: flex; justify-content: space-between; padding-bottom: 18px; }',
  '    .brand { font-size: 24px; font-weight: 700; letter-spacing: 0; }',
  '    .subtitle { color: #52606d; margin-top: 4px; }',
  '    .invoice { text-align: right; }',
  '    .invoice h1 { font-size: 22px; margin: 0 0 6px; }',
  '    .meta { display: grid; gap: 14px; grid-template-columns: 1fr 1fr; margin: 22px 0; }',
  '    .box { border: 1px solid #d9e2ec; border-radius: 6px; padding: 14px; }',
  '    .label { color: #52606d; font-size: 11px; font-weight: 700; text-transform: uppercase; }',
  '    .value { margin-top: 3px; }',
  '    table { border-collapse: collapse; margin-top: 18px; width: 100%; }',
  '    th { background: #22304a; color: #ffffff; font-size: 12px; padding: 9px; text-align: left; }',
  '    td { border-bottom: 1px solid #e4e7eb; padding: 9px; vertical-align: top; }',
  '    .num { text-align: right; white-space: nowrap; }',
  '    .total { display: flex; justify-content: flex-end; margin-top: 18px; }',
  '    .total-box { border-top: 2px solid #22304a; min-width: 260px; padding-top: 10px; }',
  '    .total-row { display: flex; font-size: 18px; font-weight: 700; justify-content: space-between; }',
  '    .foot { color: #52606d; font-size: 11px; margin-top: 28px; }',
  '  </style>',
  '</head>',
  '<body>',
  '  <section class="top">',
  '    <div>',
  '      <div class="brand">FICCT Boutique</div>',
  '      <div class="subtitle">Factura generada automaticamente</div>',
  '    </div>',
  '    <div class="invoice">',
  '      <h1>Factura</h1>',
  '      <div><strong>' + esc(orderCode) + '</strong></div>',
  '      <div>' + esc(issuedAt) + '</div>',
  '    </div>',
  '  </section>',
  '',
  '  <section class="meta">',
  '    <div class="box">',
  '      <div class="label">Cliente</div>',
  '      <div class="value">' + esc(payload.customer.name) + '</div>',
  '      <div class="value">' + esc(payload.customer.email) + '</div>',
  '    </div>',
  '    <div class="box">',
  '      <div class="label">Venta y pedido</div>',
  '      <div class="value">Venta: ' + esc(payload.sale_id) + '</div>',
  '      <div class="value">Pedido: ' + esc(payload.order_id) + '</div>',
  '      <div class="value">Sucursal: ' + esc(payload.branch_id) + '</div>',
  '    </div>',
  '  </section>',
  '',
  '  <table>',
  '    <thead>',
  '      <tr>',
  '        <th>#</th>',
  '        <th>Articulo</th>',
  '        <th class="num">Cantidad</th>',
  '        <th class="num">Precio unitario</th>',
  '        <th class="num">Importe</th>',
  '      </tr>',
  '    </thead>',
  '    <tbody>' + rows + '</tbody>',
  '  </table>',
  '',
  '  <section class="total">',
  '    <div class="total-box">',
  '      <div class="total-row">',
  '        <span>Total</span>',
  '        <span>' + money(payload.total) + ' ' + esc(payload.currency) + '</span>',
  '      </div>',
  '    </div>',
  '  </section>',
  '',
  '  <div class="foot">',
  '    Este documento fue generado automaticamente por FICCT Boutique al confirmar la venta.',
  '  </div>',
  '</body>',
  '</html>',
].join('\n');

const htmlBinary = await this.helpers.prepareBinaryData(Buffer.from(html, 'utf8'), 'index.html', 'text/html');

return [{
  json: {
    ...payload,
    invoice_html: html,
    invoice_file_base: invoiceFileBase,
    invoice_filename: invoiceFilename,
    invoice_issued_at: issuedAt,
  },
  binary: {
    html: htmlBinary,
  },
}];
`.trim();

const computePdfCode = String.raw`
const crypto = require('crypto');

function env(name, fallback = '') {
  if (typeof $env !== 'undefined' && $env?.[name]) return $env[name];
  if (typeof process !== 'undefined' && process.env?.[name]) return process.env[name];
  return fallback;
}

const item = $input.first();
if (!item.binary?.pdf) {
  throw new Error('Gotenberg did not return binary PDF data in field "pdf"');
}

const pdf = await this.helpers.getBinaryDataBuffer(0, 'pdf');
if (pdf.length < 100 || pdf.subarray(0, 4).toString('utf8') !== '%PDF') {
  throw new Error('Generated file is not a valid PDF');
}

const binary = { ...item.binary };
delete binary.html;
binary.pdf = {
  ...binary.pdf,
  fileName: item.json.invoice_filename,
  mimeType: 'application/pdf',
};

const pdfSha256 = crypto.createHash('sha256').update(pdf).digest('hex');
const pdfSizeBytes = pdf.length;
const documentTitle = 'Factura ' + item.json.order_code + ' - FICCT Boutique';
const loginRequest = {
  query: 'mutation Login($input: LoginInput!) { login(input: $input) { accessToken } }',
  variables: {
    input: {
      email: env('FICCT_N8N_SERVICE_EMAIL', 'staff@ficct.local'),
      password: env('FICCT_N8N_SERVICE_PASSWORD', 'Staff123!'),
    },
  },
};
const uploadRequest = {
  title: documentTitle,
  description: 'Factura generada automaticamente para pedido ' + item.json.order_code,
  category: 'pdf',
  mimeType: 'application/pdf',
  sizeBytes: pdfSizeBytes,
  metadata: {
    event: item.json.event,
    saleId: item.json.sale_id,
    orderId: item.json.order_id,
    orderCode: item.json.order_code,
    branchId: item.json.branch_id,
    customer: item.json.customer,
    pdfSha256,
  },
};

return [{
  json: {
    ...item.json,
    pdf_sha256: pdfSha256,
    pdf_size_bytes: pdfSizeBytes,
    document_title: documentTitle,
    go_login_json: JSON.stringify(loginRequest),
    ms3_upload_json: JSON.stringify(uploadRequest),
    ms3_confirm_json: JSON.stringify({ sha256: pdfSha256 }),
  },
  binary,
}];
`.trim();

const extractTokenCode = String.raw`
const item = $input.first();
const token = item.json.data?.login?.accessToken;
if (!token) {
  throw new Error('Go service account login did not return data.login.accessToken');
}

return [{
  json: {
    ...item.json,
    access_token: token,
  },
  binary: item.binary,
}];
`.trim();

const prepareEmailCode = String.raw`
const item = $input.first();
if (item.json.intact !== true || item.json.chainIntact !== true) {
  throw new Error('MS3 hash ledger verification failed for invoice document');
}
if (item.json.document?.status !== 'active') {
  throw new Error('MS3 invoice document is not active after confirmation');
}
if (!item.binary?.pdf) {
  throw new Error('Invoice PDF binary is missing before email send');
}
if (!item.json.customer?.email) {
  throw new Error('Missing customer.email before email send');
}

const emailHtml = [
  '<p>Hola ' + item.json.customer.name + ',</p>',
  '<p>Adjuntamos la factura de tu pedido <strong>' + item.json.order_code + '</strong> en FICCT Boutique.</p>',
  '<p><strong>Total:</strong> ' + Number(item.json.total).toFixed(2) + ' ' + item.json.currency + '</p>',
  '<p>Gracias por tu compra.</p>',
].join('\n');

return [{
  json: {
    ...item.json,
    email_subject: 'Factura ' + item.json.order_code + ' - FICCT Boutique',
    email_html: emailHtml,
  },
  binary: item.binary,
}];
`.trim();

const nodes = [
  {
    parameters: {
      httpMethod: 'POST',
      path: 'ficct-invoice',
      responseMode: 'onReceived',
      options: {
        rawBody: true,
        responseData: '{"status":"accepted"}',
      },
    },
    id: '5c8f266f-41a8-4ced-b4e4-778d04c04743',
    name: 'Webhook sale.confirmed',
    type: 'n8n-nodes-base.webhook',
    typeVersion: 2.1,
    position: [0, 0],
    webhookId: 'ficct-invoice',
  },
  {
    parameters: {
      mode: 'runOnceForAllItems',
      language: 'javaScript',
      jsCode: validateHmacCode,
    },
    id: '5d093633-cba5-48e5-85b5-7df75eedf392',
    name: 'Validate HMAC and Payload',
    type: 'n8n-nodes-base.code',
    typeVersion: 2,
    position: [260, 0],
  },
  {
    parameters: {
      mode: 'runOnceForAllItems',
      language: 'javaScript',
      jsCode: buildHtmlCode,
    },
    id: 'a2247df2-e298-4719-ae69-cdad9f74c365',
    name: 'Build Invoice HTML',
    type: 'n8n-nodes-base.code',
    typeVersion: 2,
    position: [520, 0],
  },
  {
    parameters: {
      method: 'POST',
      url: "={{($env.GOTENBERG_URL || 'http://gotenberg:3000') + '/forms/chromium/convert/html'}}",
      sendHeaders: true,
      specifyHeaders: 'keypair',
      headerParameters: {
        parameters: [
          {
            name: 'Gotenberg-Output-Filename',
            value: '={{$json.invoice_file_base}}',
          },
        ],
      },
      sendBody: true,
      contentType: 'multipart-form-data',
      bodyParameters: {
        parameters: [
          {
            parameterType: 'formBinaryData',
            name: 'files',
            inputDataFieldName: 'html',
          },
        ],
      },
      options: {
        response: {
          response: {
            responseFormat: 'file',
            outputPropertyName: 'pdf',
          },
        },
      },
    },
    id: '71a8dd8e-0a8f-4d31-9e7c-86f23fb0861a',
    name: 'Generate PDF with Gotenberg',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [780, 0],
  },
  {
    parameters: {
      mode: 'runOnceForAllItems',
      language: 'javaScript',
      jsCode: computePdfCode,
    },
    id: '9eb0edfb-db7c-4b76-a82c-21e4715f6c1b',
    name: 'Compute PDF SHA-256',
    type: 'n8n-nodes-base.code',
    typeVersion: 2,
    position: [1040, 0],
  },
  {
    parameters: {
      method: 'POST',
      url: "={{$env.GO_CORE_GRAPHQL_URL || 'http://go-core:8080/graphql'}}",
      sendBody: true,
      contentType: 'json',
      specifyBody: 'json',
      jsonBody: '={{$json.go_login_json}}',
      options: {
        response: {
          response: {
            responseFormat: 'json',
          },
        },
      },
    },
    id: '94c1e808-b5de-4041-8b10-3fa0ea4906bc',
    name: 'Login Go Service Account',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [1300, -160],
  },
  {
    parameters: {
      mode: 'combine',
      combineBy: 'combineByPosition',
      numberInputs: 2,
      options: {
        clashHandling: {
          values: {
            resolveClash: 'preferInput1',
            mergeMode: 'deepMerge',
            overrideEmpty: false,
          },
        },
      },
    },
    id: '42c0d1b2-8452-4e13-8d86-7dd533a5f4d8',
    name: 'Merge PDF and Login',
    type: 'n8n-nodes-base.merge',
    typeVersion: 3.2,
    position: [1560, 0],
  },
  {
    parameters: {
      mode: 'runOnceForAllItems',
      language: 'javaScript',
      jsCode: extractTokenCode,
    },
    id: '450a3a0a-3d46-4129-b21b-dfcfd3b5e2ee',
    name: 'Extract Access Token',
    type: 'n8n-nodes-base.code',
    typeVersion: 2,
    position: [1820, 0],
  },
  {
    parameters: {
      method: 'POST',
      url: "={{($env.EXPRESS_DOCS_URL || 'http://express-docs:8081') + '/api/v1/documents/upload-request'}}",
      sendHeaders: true,
      specifyHeaders: 'keypair',
      headerParameters: {
        parameters: [
          {
            name: 'Authorization',
            value: "={{'Bearer ' + $json.access_token}}",
          },
        ],
      },
      sendBody: true,
      contentType: 'json',
      specifyBody: 'json',
      jsonBody: '={{$json.ms3_upload_json}}',
      options: {
        response: {
          response: {
            responseFormat: 'json',
          },
        },
      },
    },
    id: 'c8e1557e-8ca2-4991-b632-8dd54df370d1',
    name: 'MS3 Upload Request',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [2080, -160],
  },
  {
    parameters: {
      mode: 'combine',
      combineBy: 'combineByPosition',
      numberInputs: 2,
      options: {
        clashHandling: {
          values: {
            resolveClash: 'preferInput1',
            mergeMode: 'deepMerge',
            overrideEmpty: false,
          },
        },
      },
    },
    id: '59da092c-aa7b-4c6c-9344-ed423b453652',
    name: 'Merge Upload Request',
    type: 'n8n-nodes-base.merge',
    typeVersion: 3.2,
    position: [2340, 0],
  },
  {
    parameters: {
      method: 'PUT',
      url: '={{$json.upload.url}}',
      sendHeaders: true,
      specifyHeaders: 'json',
      jsonHeaders: "={{JSON.stringify($json.upload.headers || {'Content-Type':'application/pdf'})}}",
      sendBody: true,
      contentType: 'binaryData',
      inputDataFieldName: 'pdf',
      options: {
        response: {
          response: {
            responseFormat: 'text',
            outputPropertyName: 'put_response',
          },
        },
      },
    },
    id: 'f4a59a16-483b-46f8-9850-7495b60b7bd6',
    name: 'PUT PDF to Presigned URL',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [2600, -160],
  },
  {
    parameters: {
      mode: 'combine',
      combineBy: 'combineByPosition',
      numberInputs: 2,
      options: {
        clashHandling: {
          values: {
            resolveClash: 'preferInput1',
            mergeMode: 'deepMerge',
            overrideEmpty: false,
          },
        },
      },
    },
    id: 'cfc88206-77bc-4ecf-b031-6a343d312c29',
    name: 'Merge PUT Result',
    type: 'n8n-nodes-base.merge',
    typeVersion: 3.2,
    position: [2860, 0],
  },
  {
    parameters: {
      method: 'POST',
      url: "={{($env.EXPRESS_DOCS_URL || 'http://express-docs:8081') + '/api/v1/documents/' + $json.document.id + '/confirm'}}",
      sendHeaders: true,
      specifyHeaders: 'keypair',
      headerParameters: {
        parameters: [
          {
            name: 'Authorization',
            value: "={{'Bearer ' + $json.access_token}}",
          },
        ],
      },
      sendBody: true,
      contentType: 'json',
      specifyBody: 'json',
      jsonBody: '={{$json.ms3_confirm_json}}',
      options: {
        response: {
          response: {
            responseFormat: 'json',
          },
        },
      },
    },
    id: 'b38c760a-db29-4b53-a57f-6d4678d86f92',
    name: 'MS3 Confirm Upload',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [3120, -160],
  },
  {
    parameters: {
      mode: 'combine',
      combineBy: 'combineByPosition',
      numberInputs: 2,
      options: {
        clashHandling: {
          values: {
            resolveClash: 'preferInput2',
            mergeMode: 'deepMerge',
            overrideEmpty: false,
          },
        },
      },
    },
    id: '9f7a3c70-b860-4da4-aa15-8b2df8e78b3f',
    name: 'Merge Confirm Result',
    type: 'n8n-nodes-base.merge',
    typeVersion: 3.2,
    position: [3380, 0],
  },
  {
    parameters: {
      method: 'GET',
      url: "={{($env.EXPRESS_DOCS_URL || 'http://express-docs:8081') + '/api/v1/documents/' + $json.document.id + '/verify'}}",
      sendHeaders: true,
      specifyHeaders: 'keypair',
      headerParameters: {
        parameters: [
          {
            name: 'Authorization',
            value: "={{'Bearer ' + $json.access_token}}",
          },
        ],
      },
      options: {
        response: {
          response: {
            responseFormat: 'json',
          },
        },
      },
    },
    id: '30658724-ff48-4b7d-bcd4-3a42c4b5cbfe',
    name: 'MS3 Verify Hash Ledger',
    type: 'n8n-nodes-base.httpRequest',
    typeVersion: 4.2,
    position: [3640, -160],
  },
  {
    parameters: {
      mode: 'combine',
      combineBy: 'combineByPosition',
      numberInputs: 2,
      options: {
        clashHandling: {
          values: {
            resolveClash: 'preferInput2',
            mergeMode: 'deepMerge',
            overrideEmpty: false,
          },
        },
      },
    },
    id: '889f7dd4-3fa8-49c4-87bb-bbfddca9c555',
    name: 'Merge Verify Result',
    type: 'n8n-nodes-base.merge',
    typeVersion: 3.2,
    position: [3900, 0],
  },
  {
    parameters: {
      mode: 'runOnceForAllItems',
      language: 'javaScript',
      jsCode: prepareEmailCode,
    },
    id: 'b9f7f6ac-cef7-4e94-98b9-ed6c86c59f59',
    name: 'Prepare Invoice Email',
    type: 'n8n-nodes-base.code',
    typeVersion: 2,
    position: [4160, 0],
  },
  {
    parameters: {
      resource: 'email',
      operation: 'send',
      fromEmail: "={{$env.FICCT_INVOICE_FROM_EMAIL || 'facturas@ficct.local'}}",
      toEmail: '={{$json.customer.email}}',
      subject: '={{$json.email_subject}}',
      emailFormat: 'html',
      html: '={{$json.email_html}}',
      options: {
        attachments: 'pdf',
        appendAttribution: false,
      },
    },
    id: '3a7489a3-57dc-4fb5-aa4a-cd084bc0210b',
    name: 'Send Invoice Email',
    type: 'n8n-nodes-base.emailSend',
    typeVersion: 2.1,
    position: [4420, 0],
    credentials: {
      smtp: {
        id: 'ficct-mailpit-smtp',
        name: 'FICCT Mailpit SMTP',
      },
    },
  },
];

const workflow = {
  name: 'FICCT Boutique Invoice Automation',
  nodes,
  pinData: {},
  connections: {
    'Webhook sale.confirmed': {
      main: [[{ node: 'Validate HMAC and Payload', type: 'main', index: 0 }]],
    },
    'Validate HMAC and Payload': {
      main: [[{ node: 'Build Invoice HTML', type: 'main', index: 0 }]],
    },
    'Build Invoice HTML': {
      main: [[{ node: 'Generate PDF with Gotenberg', type: 'main', index: 0 }]],
    },
    'Generate PDF with Gotenberg': {
      main: [[{ node: 'Compute PDF SHA-256', type: 'main', index: 0 }]],
    },
    'Compute PDF SHA-256': {
      main: [[
        { node: 'Login Go Service Account', type: 'main', index: 0 },
        { node: 'Merge PDF and Login', type: 'main', index: 0 },
      ]],
    },
    'Login Go Service Account': {
      main: [[{ node: 'Merge PDF and Login', type: 'main', index: 1 }]],
    },
    'Merge PDF and Login': {
      main: [[{ node: 'Extract Access Token', type: 'main', index: 0 }]],
    },
    'Extract Access Token': {
      main: [[
        { node: 'MS3 Upload Request', type: 'main', index: 0 },
        { node: 'Merge Upload Request', type: 'main', index: 0 },
      ]],
    },
    'MS3 Upload Request': {
      main: [[{ node: 'Merge Upload Request', type: 'main', index: 1 }]],
    },
    'Merge Upload Request': {
      main: [[
        { node: 'PUT PDF to Presigned URL', type: 'main', index: 0 },
        { node: 'Merge PUT Result', type: 'main', index: 0 },
      ]],
    },
    'PUT PDF to Presigned URL': {
      main: [[{ node: 'Merge PUT Result', type: 'main', index: 1 }]],
    },
    'Merge PUT Result': {
      main: [[
        { node: 'MS3 Confirm Upload', type: 'main', index: 0 },
        { node: 'Merge Confirm Result', type: 'main', index: 0 },
      ]],
    },
    'MS3 Confirm Upload': {
      main: [[{ node: 'Merge Confirm Result', type: 'main', index: 1 }]],
    },
    'Merge Confirm Result': {
      main: [[
        { node: 'MS3 Verify Hash Ledger', type: 'main', index: 0 },
        { node: 'Merge Verify Result', type: 'main', index: 0 },
      ]],
    },
    'MS3 Verify Hash Ledger': {
      main: [[{ node: 'Merge Verify Result', type: 'main', index: 1 }]],
    },
    'Merge Verify Result': {
      main: [[{ node: 'Prepare Invoice Email', type: 'main', index: 0 }]],
    },
    'Prepare Invoice Email': {
      main: [[{ node: 'Send Invoice Email', type: 'main', index: 0 }]],
    },
  },
  active: true,
  settings: {
    executionOrder: 'v1',
    saveExecutionProgress: true,
    saveManualExecutions: true,
    callerPolicy: 'workflowsFromSameOwner',
  },
  versionId: 'd595ed26-8686-4ff9-850c-c39bf36a7831d',
  meta: {
    templateCredsSetupCompleted: false,
  },
  id: 'ficct-invoice-workflow',
  tags: [],
};

writeFileSync(resolve(here, 'ficct-invoice-workflow.json'), JSON.stringify(workflow, null, 2) + '\n');
