# Twilio Setup Guide

Twilio is used as a SIP-to-PSTN bridge: it connects the doorbell's VoIP
call to your regular phone number. You need a Twilio account, a phone
number, a SIP domain, and a TwiML Bin.

## How the call flow works

```
Button press
  → sonnette POSTs to Twilio REST API: "call +33612345678 using TwiML"
  → Twilio calls your phone
  → You pick up
  → TwiML says: <Dial><Sip>sonnette01@your-domain.sip.twilio.com;transport=tls;secure=true</Sip></Dial>
  → Twilio sends SIP INVITE to pjsua (already registered)
  → pjsua auto-answers
  → Two-way audio between your phone and the doorbell
```

## Step 1: Create a Twilio account

Go to https://www.twilio.com/try-twilio and sign up.

On the dashboard, note your **Account SID** (starts with `AC`) and
**Auth Token**. These go into `sonnette.conf` as `TWILIO_ACCOUNT_SID`
and `TWILIO_AUTH_TOKEN`.

## Step 2: Get a phone number

Go to **Phone Numbers → Manage → Buy a number**.

This is the number the doorbell uses as caller ID when it calls you.
The choice of country matters a lot for cost:

**The important thing:** what matters is not only the monthly number fee,
it's the **per-minute call rate**. When the doorbell calls your phone,
Twilio charges you for the outbound leg. The rate depends on the country
of the Twilio number AND the country of your phone:

- **Same country / same zone (e.g. FR number → FR mobile):** ~4 cts/min
- **Cross-border within EEA:** similar, ~4 cts/min
- **Outside your zone (e.g. US number → FR mobile, or FR number → non-EU):**
  15+ cts/min — 3x as expensive

**Regulatory requirements by country:**
- **France (09xx):** ~1.20 EUR/month. Requires identity verification
  (name, address, ID document). Validation takes 24-48h. Straightforward.
- **US/UK:** ~1 USD/month. Available instantly, no ID required, but high
  price to EU countries.
- **Czech Republic:** cheap, no ID required, and calls to EU/FR mobiles
  are roughly the same price as a local FR number. Good option to avoid
  the identity paperwork for EU residents.

Note the number (e.g. `+33987654321`). This goes into `sonnette.conf`
as `TWILIO_FROM_NUMBER`.

Set `TWILIO_TO_NUMBER` to your personal phone number (the one you want
to ring when someone presses the doorbell).

## Step 3: Create a SIP Domain

On the Account Dashboard, in the left menu, go to **Voice → Manage →
SIP Domains** (or search "SIP" in the console).

- Click **Create SIP Domain**
- Choose a name (e.g. `sonnette`). The full domain will be
  `sonnette.sip.us1.twilio.com` (or another region).
- Under **Voice Authentication**, add a credential list:
  - Create a new credential list
  - Add a username/password (e.g. `sonnette01` / `your-strong-password`)
  - This is what pjsua uses to register
- **Enable Secure Media** — required for TLS signaling + SRTP encrypted
  audio. Without this, Twilio will not negotiate `a=crypto:` in the SDP
  and the call audio goes in clear over the internet.
- Leave Emergency Calling disabled (not used here).
- Enable **SIP Registration** & add your credential list
- Hit save at the bottom of the page

These go into `sonnette.conf`:
```
SIP_USERNAME="sonnette01"
SIP_PASSWORD="your-strong-password"
SIP_DOMAIN="sonnette.sip.us1.twilio.com"
SIP_REALM="sip.twilio.com"    # always this for Twilio
```

## Step 4: Create a TwiML Bin

Still on the Account Dashboard, go to **Explore Products → TwiML Bins**.

- Click **Create TwiML Bin**
- Name: e.g. "Sonnette handler"
- Paste this TwiML (replace with your SIP domain and username).
  The `;transport=tls;secure=true` parameters are **required** for
  encrypted signaling (TLS) and audio (SRTP):
  ```xml
  <?xml version="1.0" encoding="UTF-8"?>
  <Response>
    <Dial>
      <Sip>sip:sonnette01@sonnette.sip.us1.twilio.com;transport=tls;secure=true</Sip>
    </Dial>
  </Response>
  ```
- Save and copy the URL (starts with `https://handler.twilio.com/twiml/EH...`)

This goes into `sonnette.conf` as `TWILIO_TWIML_URL`.

## Step 5: Fill in sonnette.conf

Prepare `deploy/etc/sonnette.conf` locally from
`deploy/sonnette.conf.example`, fill in all the values from the steps
above, then deploy it with the ADB flow in
[docs/deploy.md](deploy.md#3-deploy):

```bash
mkdir -p deploy/etc
cp deploy/sonnette.conf.example deploy/etc/sonnette.conf
# edit deploy/etc/sonnette.conf
adb push deploy/etc/sonnette.conf /etc/sonnette.conf
adb shell "chmod 0600 /etc/sonnette.conf"
```

Reference:

| Variable | Where to find it |
|----------|-----------------|
| `TWILIO_ACCOUNT_SID` | Dashboard → Account SID |
| `TWILIO_AUTH_TOKEN` | Dashboard → Auth Token |
| `TWILIO_FROM_NUMBER` | Phone Numbers → your Twilio number |
| `TWILIO_TO_NUMBER` | Your personal phone number |
| `TWILIO_TWIML_URL` | TwiML Bins → your bin URL |
| `SIP_USERNAME` | SIP Domains → Credential List → username |
| `SIP_PASSWORD` | SIP Domains → Credential List → password |
| `SIP_DOMAIN` | SIP Domains → your domain FQDN |
| `SIP_REALM` | Always `sip.twilio.com` |

## Cost estimate

| Item | Cost |
|------|------|
| French phone number (09xx) | ~1.20 EUR/month |
| Outbound call (doorbell → your phone) | ~0.04 EUR/min |
| SIP registration | Free |
| Twilio account | Free (pay-as-you-go) |
