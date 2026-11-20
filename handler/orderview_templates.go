package handler

const pageStyle = `
	:root {
		--ink: #0b0b0a;
		--paper: #f2efe8;
		--card: #ffffff;
		--line: rgba(11, 11, 10, .14);
		--muted: #726d64;
		--red: #c63a2f;
		--ok: #15803d;
		--wait: #b45309;
		--bad: #b91c1c;
		--ok-bg: #e8f2e9;
		--wait-bg: #fbf1df;
		--bad-bg: #fbe9e7;
		--soft: #e9e5dc;
	}
	* { box-sizing: border-box; }
	body {
		margin: 0;
		background: var(--paper);
		color: var(--ink);
		font: 14px/1.6 'Inter', system-ui, -apple-system, 'Segoe UI', sans-serif;
		-webkit-font-smoothing: antialiased;
	}
	a { color: inherit; }
	.topbar { background: var(--ink); color: var(--paper); }
	.topbar-inner {
		max-width: 1040px;
		margin: 0 auto;
		padding: 18px 24px;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.wordmark {
		display: inline-flex;
		align-items: center;
		text-decoration: none;
	}
	.wordmark-img { display: block; height: 24px; width: auto; }
	.site-footer { background: var(--ink); color: var(--paper); margin-top: 40px; }
	.footer-inner { max-width: 1040px; margin: 0 auto; padding: 40px 24px 24px; }
	.footer-wordmark {
		display: block;
		width: 100%;
		height: auto;
		max-height: 220px;
		object-fit: contain;
		object-position: left center;
	}
	.footer-bottom {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		border-top: 1px solid rgba(242, 239, 232, .12);
		margin-top: 28px;
		padding-top: 16px;
		font-size: 9px;
		letter-spacing: .14em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .4);
	}
	.footer-nav { display: flex; gap: 18px; }
	.footer-nav a { color: inherit; text-decoration: none; }
	.footer-nav a:hover { color: var(--paper); }
	.footer-bottom p { margin: 0; }
	.topnav {
		display: flex;
		gap: 18px;
		font-size: 9px;
		letter-spacing: .16em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .6);
	}
	.topnav a { color: inherit; text-decoration: none; }
	.topnav a:hover { color: var(--paper); }
	.wrap { max-width: 1040px; margin: 0 auto; padding: 28px 24px 64px; }
	.hero {
		background: var(--ink);
		color: var(--paper);
		padding: 32px;
		position: relative;
		overflow: hidden;
	}
	.hero-top {
		display: flex;
		justify-content: space-between;
		gap: 24px;
		align-items: flex-start;
		position: relative;
		z-index: 1;
	}
	.eyebrow {
		margin: 0;
		font-size: 9px;
		letter-spacing: .2em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .45);
	}
	.hero h1 {
		font-family: 'Old Standard TT', Georgia, serif;
		font-size: 46px;
		line-height: .95;
		text-transform: uppercase;
		margin: 14px 0 0;
		letter-spacing: -.01em;
	}
	.hero .sub {
		margin: 12px 0 0;
		max-width: 440px;
		color: rgba(242, 239, 232, .6);
		font-size: 13px;
		line-height: 1.7;
	}
	.badge {
		display: inline-block;
		padding: 6px 10px;
		border-radius: 999px;
		font-size: 9px;
		font-weight: 600;
		letter-spacing: .12em;
		text-transform: uppercase;
		white-space: nowrap;
	}
	.ok { background: var(--ok-bg); color: var(--ok); }
	.wait { background: var(--wait-bg); color: var(--wait); }
	.bad { background: var(--bad-bg); color: var(--bad); }
	.muted { background: var(--soft); color: var(--muted); }
	.steps {
		list-style: none;
		display: grid;
		grid-template-columns: repeat(5, 1fr);
		margin: 32px 0 0;
		padding: 0;
		position: relative;
		z-index: 1;
	}
	.step {
		position: relative;
		text-align: center;
		font-size: 8px;
		letter-spacing: .08em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .35);
	}
	.step:not(:last-child)::after {
		content: '';
		position: absolute;
		top: 5px;
		left: calc(50% + 7px);
		width: calc(100% - 14px);
		height: 1px;
		background: rgba(242, 239, 232, .18);
	}
	.step.done:not(:last-child)::after { background: var(--red); }
	.step-dot {
		display: block;
		width: 11px;
		height: 11px;
		margin: 0 auto 8px;
		border-radius: 50%;
		border: 1px solid rgba(242, 239, 232, .35);
		background: transparent;
	}
	.step.done .step-dot { background: var(--red); border-color: var(--red); }
	.step.current .step-dot {
		border-color: var(--paper);
		box-shadow: 0 0 0 3px rgba(242, 239, 232, .12);
	}
	.step.done, .step.current { color: var(--paper); }
	.hero-meta {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		margin-top: 24px;
		padding-top: 14px;
		border-top: 1px solid rgba(242, 239, 232, .14);
		font-size: 9px;
		letter-spacing: .12em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .4);
		position: relative;
		z-index: 1;
	}
	.layout {
		display: grid;
		grid-template-columns: 1.35fr .65fr;
		gap: 18px;
		margin-top: 18px;
		align-items: start;
	}
	.col { display: grid; gap: 18px; }
	.card { background: var(--card); border: 1px solid var(--line); }
	.card-pad { padding: 20px; }
	.label {
		margin: 0 0 12px;
		font-size: 9px;
		letter-spacing: .16em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.map-card { overflow: hidden; }
	#route-map { height: 360px; width: 100%; background: var(--soft); }
	.map-meta {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		padding: 12px 16px;
		border-top: 1px solid var(--line);
		font-size: 9px;
		letter-spacing: .12em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.map-meta span { display: flex; align-items: center; gap: 6px; }
	.map-dot { display: inline-block; width: 6px; height: 6px; border-radius: 50%; }
	.timeline { list-style: none; margin: 0; padding: 0; }
	.event {
		position: relative;
		padding: 0 0 18px 20px;
		border-left: 1px solid var(--line);
		margin-left: 5px;
	}
	.event:last-child { padding-bottom: 0; border-left-color: transparent; }
	.event::before {
		content: '';
		position: absolute;
		left: -4px;
		top: 4px;
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--line);
	}
	.event.done::before { background: var(--ink); }
	.event.current::before { background: var(--red); box-shadow: 0 0 0 3px rgba(198, 58, 47, .12); }
	.event.muted::before { background: var(--muted); }
	.event.bad::before { background: var(--bad); }
	.event-title { font-size: 12px; font-weight: 600; }
	.event-note { color: var(--muted); font-size: 12px; margin-top: 2px; }
	.event-when {
		color: var(--muted);
		font-size: 9px;
		letter-spacing: .08em;
		text-transform: uppercase;
		margin-top: 4px;
	}
	.kv {
		display: grid;
		grid-template-columns: 110px 1fr;
		gap: 8px 14px;
		margin: 0;
		font-size: 12px;
	}
	.kv dt { color: var(--muted); }
	.kv dd { margin: 0; word-break: break-word; }
	table { width: 100%; border-collapse: collapse; }
	th {
		text-align: left;
		font-size: 9px;
		letter-spacing: .1em;
		text-transform: uppercase;
		color: var(--muted);
		padding: 0 0 8px;
	}
	td { padding: 10px 0; border-top: 1px solid var(--line); font-size: 13px; }
	.right { text-align: right; }
	.totals { margin-top: 10px; }
	.totals div {
		display: flex;
		justify-content: space-between;
		padding: 5px 0;
		font-size: 12px;
		color: var(--muted);
	}
	.totals .grand {
		border-top: 1px solid var(--line);
		margin-top: 4px;
		padding-top: 10px;
		color: var(--ink);
		font-size: 15px;
		font-weight: 600;
	}
	.addr { margin: 0; font-size: 13px; color: var(--muted); line-height: 1.7; }
	.addr strong { color: var(--ink); font-weight: 600; }
	.actions { display: flex; flex-wrap: wrap; gap: 10px; }
	.btn {
		display: inline-block;
		background: var(--ink);
		color: var(--paper);
		padding: 11px 16px;
		text-decoration: none;
		font-size: 9px;
		letter-spacing: .14em;
		text-transform: uppercase;
		font-weight: 600;
		border: 1px solid var(--ink);
		cursor: pointer;
	}
	.btn.line { background: transparent; color: var(--ink); border-color: var(--line); }
	.note {
		background: var(--wait-bg);
		color: var(--wait);
		padding: 12px 14px;
		font-size: 12px;
		margin: 0 0 16px;
		border: 1px solid rgba(180, 83, 9, .2);
	}
	.foot {
		text-align: center;
		color: var(--muted);
		font-size: 10px;
		letter-spacing: .06em;
		margin: 28px 0 0;
	}
	.track-head {
		display: flex;
		justify-content: space-between;
		gap: 24px;
		align-items: flex-end;
		padding: 10px 0 24px;
		border-bottom: 1px solid var(--line);
	}
	.track-head h1 {
		font-family: 'Old Standard TT', Georgia, serif;
		font-size: 52px;
		line-height: .95;
		text-transform: uppercase;
		margin: 8px 0 0;
	}
	.track-head p { max-width: 420px; color: var(--muted); font-size: 13px; margin: 0; }
	.track-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 18px;
		margin-top: 18px;
		align-items: stretch;
	}
	.track-visual {
		background: var(--ink);
		color: var(--paper);
		padding: 28px;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		min-height: 360px;
	}
	.track-visual svg { width: 100%; height: auto; }
	.track-visual .caption {
		font-size: 9px;
		letter-spacing: .16em;
		text-transform: uppercase;
		color: rgba(242, 239, 232, .45);
	}
	.form-card { padding: 28px; }
	form .field { margin-bottom: 16px; }
	label {
		display: block;
		font-size: 10px;
		letter-spacing: .08em;
		text-transform: uppercase;
		color: var(--muted);
		margin-bottom: 6px;
	}
	input {
		width: 100%;
		padding: 12px;
		border: 1px solid var(--line);
		background: #fff;
		color: var(--ink);
		font: inherit;
		font-size: 14px;
		outline: none;
	}
	input:focus { border-color: var(--ink); }
	form .btn { width: 100%; padding: 13px; margin-top: 4px; }
	@media (max-width: 760px) {
		.wrap { padding: 18px 16px 48px; }
		.hero { padding: 24px 18px; }
		.hero h1 { font-size: 34px; }
		.hero-top { flex-direction: column; }
		.layout { grid-template-columns: 1fr; }
		.track-head { flex-direction: column; align-items: flex-start; }
		.track-head h1 { font-size: 40px; }
		.track-grid { grid-template-columns: 1fr; }
		.track-visual { min-height: 240px; }
		.topbar-inner { padding: 16px; }
		.topnav { gap: 12px; }
		.step { font-size: 7px; }
		#route-map { height: 300px; }
		.kv { grid-template-columns: 90px 1fr; }
		.footer-inner { padding: 28px 16px 20px; }
		.footer-bottom { flex-direction: column; }
	}
`

const orderPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
{{if .AutoRefresh}}<meta http-equiv="refresh" content="6">{{end}}
<title>Order {{short .ID.String}} — Solvun</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&amp;family=Old+Standard+TT:wght@400;700&amp;display=swap" rel="stylesheet">
{{if .HasMap}}<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css">{{end}}
<style>` + pageStyle + `</style>
</head>
<body>
<div class="topbar">
	<div class="topbar-inner">
		<a class="wordmark" href="/" aria-label="Solvun home"><img src="/images/brand/solvun-wordmark-sm.png" alt="Solvun" class="wordmark-img"></a>
		<nav class="topnav">
			<a href="/shop">Shop</a>
			<a href="/track">Track order</a>
		</nav>
	</div>
</div>

<div class="wrap">
	<div class="hero">
		<div class="hero-top">
			<div>
				<p class="eyebrow">Order tracking</p>
				<h1>{{.Headline}}</h1>
				<p class="sub">{{.Subline}}</p>
			</div>
			<span class="badge {{.Tone}}">{{.Badge}}</span>
		</div>

		<ol class="steps">
			{{range .Steps}}
			<li class="step {{.State}}">
				<span class="step-dot"></span>
				<span>{{.Label}}</span>
			</li>
			{{end}}
		</ol>

		<div class="hero-meta">
			<span>Order {{short .ID.String}}</span>
			<span>{{date .CreatedAt}}</span>
		</div>
	</div>

	<div class="layout">
		<div class="col">
			{{if .HasMap}}
			<div class="card map-card">
				<div id="route-map"></div>
				<div class="map-meta">
					<span><i class="map-dot" style="background:#0b0b0a"></i>Origin — Solvun studio</span>
					<span><i class="map-dot" style="background:#c63a2f"></i>Delivery — {{.Address.City}}</span>
				</div>
			</div>
			{{end}}

			<div class="card card-pad">
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
			</div>

			<div class="card card-pad">
				<p class="label">Shipping address</p>
				<p class="addr">
					<strong>{{.DeliveryName}}</strong><br>
					{{.Address.StreetLine}}<br>
					{{.Address.City}}, {{.Address.State}} {{.Address.ZipCode}}<br>
					{{.DeliveryPhone}}
				</p>
			</div>
		</div>

		<div class="col">
			<div class="card card-pad">
				<p class="label">Journey</p>
				<ul class="timeline">
				{{range .Events}}
					<li class="event {{.Tone}}">
						<div class="event-title">{{.Title}}</div>
						{{if .Note}}<div class="event-note">{{.Note}}</div>{{end}}
						{{if .When}}<div class="event-when">{{.When}}</div>{{end}}
					</li>
				{{end}}
				</ul>
			</div>

			<div class="card card-pad">
				<p class="label">Shipment</p>
				{{if .Shipment}}
				<dl class="kv">
					<dt>Courier</dt><dd>{{if .Courier}}{{.Courier}} {{.CourierService}}{{else}}—{{end}}</dd>
					<dt>Waybill</dt><dd>{{if .Shipment.WaybillID}}{{.Shipment.WaybillID}}{{else}}—{{end}}</dd>
					<dt>Status</dt><dd>{{if .ShipmentStatusLabel}}{{.ShipmentStatusLabel}}{{else}}—{{end}}</dd>
					{{if .Shipment.TrackingID}}<dt>Tracking ID</dt><dd>{{.Shipment.TrackingID}}</dd>{{end}}
				</dl>
				{{else}}
				<p class="addr">Not shipped yet. The waybill appears here once the order is paid and handed to the courier.</p>
				{{end}}
			</div>

			{{if .Payment}}
			<div class="card card-pad">
				<p class="label">Payment</p>
				<dl class="kv">
					<dt>Method</dt><dd>{{.Payment.MethodLabel}}</dd>
					{{if .Payment.Bank}}<dt>Bank</dt><dd>{{.Payment.Bank}}</dd>{{end}}
					{{if .Payment.VANumber}}<dt>VA number</dt><dd>{{.Payment.VANumber}}</dd>{{end}}
					{{if .Payment.BillerCode}}<dt>Biller code</dt><dd>{{.Payment.BillerCode}}</dd>{{end}}
					{{if .Payment.BillKey}}<dt>VA number</dt><dd>{{.Payment.BillKey}}</dd>{{end}}
					{{if .Payment.PaymentCode}}<dt>Payment code</dt><dd>{{.Payment.PaymentCode}}</dd>{{end}}
					{{if .Payment.Store}}<dt>Store</dt><dd>{{.Payment.Store}}</dd>{{end}}
					{{if .Payment.TransactionID}}<dt>Transaction ID</dt><dd>{{.Payment.TransactionID}}</dd>{{end}}
					{{if .Payment.SettledAt}}<dt>Settled at</dt><dd>{{.Payment.SettledAt}}</dd>{{end}}
				</dl>
			</div>
			{{end}}

			<div class="card card-pad">
				<p class="label">Actions</p>
				<div class="actions">
					{{if .PaymentURL}}{{if eq (printf "%s" .Status) "pending"}}<a class="btn" href="{{.PaymentURL}}">Pay now</a>{{end}}{{end}}
					<a class="btn line" href="/orders/{{.ID.String}}/invoice.pdf">Download invoice</a>
					<a class="btn line" href="/track">Track another order</a>
				</div>
			</div>
		</div>
	</div>

	<p class="foot">Keep this link to follow your order. Need help? Contact us quoting Order {{short .ID.String}}.</p>
</div>

<footer class="site-footer">
	<div class="footer-inner">
		<img src="/images/brand/solvun-wordmark.png" alt="Solvun" class="footer-wordmark" loading="lazy" decoding="async">
		<div class="footer-bottom">
			<nav class="footer-nav">
				<a href="/">Home</a>
				<a href="/shop">Shop</a>
				<a href="/track">Track order</a>
			</nav>
			<p>© Solvun — Secure payments by Midtrans</p>
		</div>
	</div>
</footer>

{{if .HasMap}}
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<script>
(function () {
	var store = [{{.StoreLat}}, {{.StoreLon}}];
	var destination = [{{.DestinationLat}}, {{.DestinationLon}}];
	var map = L.map('route-map', { scrollWheelZoom: false }).fitBounds([store, destination], { padding: [36, 36] });
	L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
		maxZoom: 19,
		attribution: '\u00a9 OpenStreetMap'
	}).addTo(map);
	L.polyline([store, destination], {
		color: '#c63a2f',
		weight: 2,
		dashArray: '6 8',
		opacity: 0.9
	}).addTo(map);
	function dot(color) {
		return L.divIcon({
			className: '',
			html: '<span style="display:block;width:14px;height:14px;border-radius:50%;background:' + color + ';border:3px solid #fff;box-shadow:0 2px 8px rgba(0,0,0,.3)"></span>',
			iconSize: [14, 14],
			iconAnchor: [7, 7]
		});
	}
	L.marker(store, { icon: dot('#0b0b0a') }).addTo(map).bindPopup('Solvun studio');
	L.marker(destination, { icon: dot('#c63a2f') }).addTo(map).bindPopup('Delivery address');
})();
</script>
{{end}}
</body>
</html>`

const trackPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Track your order — Solvun</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&amp;family=Old+Standard+TT:wght@400;700&amp;display=swap" rel="stylesheet">
<style>` + pageStyle + `</style>
</head>
<body>
<div class="topbar">
	<div class="topbar-inner">
		<a class="wordmark" href="/" aria-label="Solvun home"><img src="/images/brand/solvun-wordmark-sm.png" alt="Solvun" class="wordmark-img"></a>
		<nav class="topnav">
			<a href="/shop">Shop</a>
			<a href="/track">Track order</a>
		</nav>
	</div>
</div>

<div class="wrap">
	<div class="track-head">
		<div>
			<p class="eyebrow">Delivery</p>
			<h1>Find your order.</h1>
		</div>
		<p>Enter the Order ID from your confirmation email and the email address you used when ordering.</p>
	</div>

	<div class="track-grid">
		<div class="track-visual">
			<svg viewBox="0 0 420 240" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
				<path d="M34 190 C 120 190, 128 58, 210 58 S 300 190, 386 190" stroke="rgba(242,239,232,.28)" stroke-width="1.5" stroke-dasharray="4 8"/>
				<circle cx="34" cy="190" r="6" fill="#c63a2f"/>
				<circle cx="386" cy="190" r="6" fill="#f2efe8"/>
				<rect x="183" y="84" width="54" height="42" stroke="#f2efe8" stroke-width="1.2"/>
				<path d="M183 98 H237 M210 84 V126" stroke="#f2efe8" stroke-width="1.2"/>
				<path d="M197 84 V98 H223 V84" stroke="#f2efe8" stroke-width="1.2"/>
			</svg>
			<p class="caption">Solvun — After Hours / Vol. 01</p>
		</div>

		<div class="card form-card">
			{{if .Error}}<p class="note">The Order ID or email does not match. Please check and try again.</p>{{end}}
			<form method="post" action="/track">
				<div class="field">
					<label for="order_id">Order ID</label>
					<input id="order_id" name="order_id" placeholder="88e44b1e-b950-48aa-8b55-7dc0399de05f" required>
				</div>
				<div class="field">
					<label for="email">Email</label>
					<input id="email" name="email" type="email" placeholder="you@email.com" required>
				</div>
				<button class="btn" type="submit">Track order</button>
			</form>
			<p class="foot" style="text-align:left;margin-top:20px">Your Order ID is in the confirmation email. Delivery updates refresh automatically.</p>
		</div>
	</div>

	<p class="foot">Need help? Reply to your confirmation email and quote your Order ID.</p>
</div>

<footer class="site-footer">
	<div class="footer-inner">
		<img src="/images/brand/solvun-wordmark.png" alt="Solvun" class="footer-wordmark" loading="lazy" decoding="async">
		<div class="footer-bottom">
			<nav class="footer-nav">
				<a href="/">Home</a>
				<a href="/shop">Shop</a>
				<a href="/track">Track order</a>
			</nav>
			<p>© Solvun — Secure payments by Midtrans</p>
		</div>
	</div>
</footer>
</body>
</html>`
