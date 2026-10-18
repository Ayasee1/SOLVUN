package handler

const pageStyle = `
	:root {
		--bg: #f4f4f5;
		--card: #ffffff;
		--text: #18181b;
		--muted: #71717a;
		--line: #e4e4e7;
		--accent: #18181b;
		--ok-bg: #f0fdf4; --ok-fg: #15803d;
		--wait-bg: #fffbeb; --wait-fg: #b45309;
		--bad-bg: #fef2f2; --bad-fg: #b91c1c;
		--muted-bg: #f4f4f5; --muted-fg: #52525b;
	}
	@media (prefers-color-scheme: dark) {
		:root {
			--bg: #09090b;
			--card: #131316;
			--text: #f4f4f5;
			--muted: #a1a1aa;
			--line: #27272a;
			--accent: #f4f4f5;
			--ok-bg: #052e16; --ok-fg: #4ade80;
			--wait-bg: #2c1a03; --wait-fg: #fbbf24;
			--bad-bg: #2c0b0b; --bad-fg: #f87171;
			--muted-bg: #1c1c1f; --muted-fg: #a1a1aa;
		}
	}
	* { box-sizing: border-box; }
	body {
		margin: 0;
		padding: 40px 20px;
		background: var(--bg);
		color: var(--text);
		font: 14px/1.6 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
		-webkit-font-smoothing: antialiased;
	}
	.wrap { max-width: 540px; margin: 0 auto; }
	.brand {
		font-size: 12px;
		font-weight: 600;
		letter-spacing: .12em;
		text-transform: uppercase;
		color: var(--muted);
		margin: 0 0 20px;
	}
	.card {
		background: var(--card);
		border: 1px solid var(--line);
		border-radius: 12px;
		padding: 28px;
	}
	.head { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
	.head h1 { font-size: 18px; font-weight: 600; margin: 0; letter-spacing: -.01em; }
	.head p { margin: 4px 0 0; color: var(--muted); font-size: 13px; }
	.badge {
		display: inline-block;
		padding: 4px 10px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		white-space: nowrap;
	}
	.ok { background: var(--ok-bg); color: var(--ok-fg); }
	.wait { background: var(--wait-bg); color: var(--wait-fg); }
	.bad { background: var(--bad-bg); color: var(--bad-fg); }
	.muted { background: var(--muted-bg); color: var(--muted-fg); }
	hr { border: 0; border-top: 1px solid var(--line); margin: 24px 0; }
	.label {
		font-size: 11px;
		font-weight: 600;
		letter-spacing: .08em;
		text-transform: uppercase;
		color: var(--muted);
		margin: 0 0 12px;
	}
	table { width: 100%; border-collapse: collapse; }
	th {
		text-align: left;
		font-size: 11px;
		font-weight: 600;
		letter-spacing: .06em;
		text-transform: uppercase;
		color: var(--muted);
		padding: 0 0 8px;
	}
	td { padding: 10px 0; border-top: 1px solid var(--line); }
	.right { text-align: right; }
	.totals { margin-top: 4px; }
	.totals div { display: flex; justify-content: space-between; padding: 6px 0; font-size: 13px; color: var(--muted); }
	.totals .grand {
		border-top: 1px solid var(--line);
		margin-top: 6px;
		padding-top: 12px;
		color: var(--text);
		font-size: 16px;
		font-weight: 600;
	}
	dl { display: grid; grid-template-columns: 120px 1fr; gap: 8px 16px; margin: 0; font-size: 13px; }
	dt { color: var(--muted); }
	dd { margin: 0; }
	.timeline { list-style: none; margin: 12px 0 0; padding: 0 0 0 16px; border-left: 1px solid var(--line); }
	.timeline li { position: relative; padding: 0 0 12px 4px; font-size: 13px; }
	.timeline li::before {
		content: "";
		position: absolute;
		left: -21px;
		top: 7px;
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--line);
	}
	.timeline li:first-child::before { background: var(--text); }
	.timeline .when { color: var(--muted); font-size: 12px; }
	.note {
		background: var(--wait-bg);
		color: var(--wait-fg);
		border-radius: 8px;
		padding: 12px 14px;
		font-size: 13px;
		margin: 20px 0 0;
	}
	.actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 20px; }
	.btn {
		display: inline-block;
		background: var(--accent);
		color: var(--bg);
		padding: 11px 18px;
		border-radius: 8px;
		text-decoration: none;
		font-size: 13px;
		font-weight: 600;
	}
	.btn.line { background: transparent; color: var(--text); border: 1px solid var(--line); }
	.addr { font-size: 13px; color: var(--muted); margin: 0; }
	.addr strong { color: var(--text); font-weight: 600; }
	.foot { text-align: center; color: var(--muted); font-size: 12px; margin: 24px 0 0; }
	input {
		width: 100%;
		padding: 11px 12px;
		border: 1px solid var(--line);
		border-radius: 8px;
		background: var(--card);
		color: var(--text);
		font-size: 14px;
		margin-top: 6px;
	}
	label { font-size: 12px; font-weight: 500; color: var(--muted); }
	form div { margin-bottom: 16px; }
`

const orderPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
{{if eq (printf "%s" .Status) "pending"}}<meta http-equiv="refresh" content="6">{{end}}
<title>Order {{short .ID.String}} — Golearn Store</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="wrap">
	<p class="brand">Golearn Store</p>

	<div class="card">
		<div class="head">
			<div>
				<h1>Order {{short .ID.String}}</h1>
				<p>{{date .CreatedAt}}</p>
			</div>
			<span class="badge {{statusClass .Status}}">{{statusLabel .Status}}</span>
		</div>

		{{if eq (printf "%s" .Status) "pending"}}
			<p class="note">Waiting for payment confirmation. This page refreshes on its own.</p>
		{{end}}

		<hr>

		<p class="label">Items</p>
		<table>
			<thead><tr><th>Product</th><th class="right">Qty</th><th class="right">Amount</th></tr></thead>
			<tbody>
			{{range .OrderItem}}
				<tr><td>{{.NameItem}}</td><td class="right">{{.Qty}}</td><td class="right">{{rupiah .TotalPrice}}</td></tr>
			{{end}}
			</tbody>
		</table>

		<div class="totals">
			<div><span>Shipping{{if .Courier}} · {{.Courier}} {{.CourierService}}{{end}}</span><span>{{rupiah .ShippingCost}}</span></div>
			<div class="grand"><span>Total</span><span>{{rupiah .Total}}</span></div>
		</div>

		<hr>

		<p class="label">Shipment</p>
		{{if .Shipment}}
			<dl>
				<dt>Waybill</dt><dd>{{if .Shipment.WaybillID}}{{.Shipment.WaybillID}}{{else}}—{{end}}</dd>
				<dt>Status</dt><dd>{{.Shipment.Status}}</dd>
			</dl>
			{{if .Shipment.History}}
				<ul class="timeline">
				{{range .Shipment.History}}
					<li>{{.Note}}<div class="when">{{.UpdatedAt}}</div></li>
				{{end}}
				</ul>
			{{end}}
		{{else}}
			<p class="addr">Not shipped yet. The waybill appears here once the order is paid and handed to the courier.</p>
		{{end}}

		{{if .Payment}}
		<hr>
		<p class="label">Payment</p>
		<dl>
			<dt>Method</dt><dd>{{.Payment.MethodLabel}}</dd>
			{{if .Payment.Bank}}<dt>Bank</dt><dd>{{.Payment.Bank}}</dd>{{end}}
			{{if .Payment.VANumber}}<dt>VA number</dt><dd>{{.Payment.VANumber}}</dd>{{end}}
			{{if .Payment.BillerCode}}<dt>Biller code</dt><dd>{{.Payment.BillerCode}}</dd>{{end}}
			{{if .Payment.BillKey}}<dt>VA number</dt><dd>{{.Payment.BillKey}}</dd>{{end}}
			{{if .Payment.PaymentCode}}<dt>Payment code</dt><dd>{{.Payment.PaymentCode}}</dd>{{end}}
			{{if .Payment.Store}}<dt>Store</dt><dd>{{.Payment.Store}}</dd>{{end}}
			{{if .Payment.TransactionID}}<dt>Transaction ID</dt><dd style="word-break:break-all">{{.Payment.TransactionID}}</dd>{{end}}
			{{if .Payment.SettledAt}}<dt>Settled at</dt><dd>{{.Payment.SettledAt}}</dd>{{end}}
		</dl>
		{{end}}

		<hr>

		<p class="label">Shipping address</p>
		<p class="addr">
			<strong>{{.DeliveryName}}</strong><br>
			{{.Address.StreetLine}}<br>
			{{.Address.City}}, {{.Address.State}} {{.Address.ZipCode}}<br>
			{{.DeliveryPhone}}
		</p>

		<div class="actions">
			{{if .PaymentURL}}{{if eq (printf "%s" .Status) "pending"}}<a class="btn" href="{{.PaymentURL}}">Pay now</a>{{end}}{{end}}
			<a class="btn line" href="/orders/{{.ID.String}}/invoice.pdf">Download invoice</a>
		</div>
	</div>

	<p class="foot">Keep this link to follow your order. Need help? Contact us quoting the Order ID above.</p>
</div>
</body>
</html>`

const trackPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Track your order — Golearn Store</title>
<style>` + pageStyle + `</style>
</head>
<body>
<div class="wrap">
	<p class="brand">Golearn Store</p>
	<div class="card">
		<h1 style="font-size:18px;font-weight:600;margin:0 0 6px;letter-spacing:-.01em">Track your order</h1>
		<p style="color:var(--muted);font-size:13px;margin:0 0 20px">
			Enter the Order ID from your confirmation email and the email address you used when ordering.
		</p>

		{{if .Error}}<p class="note" style="margin:0 0 20px">The Order ID or email does not match. Please check and try again.</p>{{end}}

		<form method="post" action="/track">
			<div>
				<label for="order_id">Order ID</label>
				<input id="order_id" name="order_id" placeholder="88e44b1e-b950-48aa-8b55-7dc0399de05f" required>
			</div>
			<div>
				<label for="email">Email</label>
				<input id="email" name="email" type="email" placeholder="you@email.com" required>
			</div>
			<button class="btn" type="submit">Track order</button>
		</form>
	</div>
</div>
</body>
</html>`
