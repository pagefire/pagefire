# Twilio SMS & Phone Call Setup

PageFire can send SMS and phone call notifications through Twilio. This guide walks you through connecting your Twilio account.

## Prerequisites

- A Twilio account ([sign up here](https://www.twilio.com/try-twilio))
- A Twilio phone number with **SMS** and **Voice** capabilities
- A running PageFire instance

## Step 1: Get Your Twilio Credentials

1. Log in to the [Twilio Console](https://console.twilio.com/).
2. On the dashboard, copy your **Account SID** and **Auth Token**.

## Step 2: Get a Twilio Phone Number

If you don't already have one:

1. In the Twilio Console, go to **Phone Numbers > Manage > Buy a number**.
2. Select a number with both **SMS** and **Voice** capabilities.
3. Copy the phone number (it will be in E.164 format, e.g. `+12025551234`).

> **Trial accounts** come with a free number, but can only send to verified numbers. You must verify each recipient number under **Phone Numbers > Manage > Verified Caller IDs** before it will work.

## Step 3: Set Environment Variables

Configure PageFire with your Twilio credentials:

```bash
export PAGEFIRE_TWILIO_ACCOUNT_SID="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export PAGEFIRE_TWILIO_AUTH_TOKEN="your-auth-token"
export PAGEFIRE_TWILIO_FROM_NUMBER="+12025551234"
```

The `FROM_NUMBER` must be in E.164 format: `+` followed by the country code and number, no spaces or dashes.

## Step 4: Restart PageFire

```bash
# If running directly
pagefire serve

# If running via Docker
docker restart pagefire
```

PageFire picks up environment variables at startup. The SMS and phone call providers are automatically enabled when all three Twilio variables are set.

## Step 5: Add Contact Methods

1. Log in to PageFire and go to **Profile & Settings**.
2. Under **Contact Methods**, click **Add Contact Method**.
3. Select **SMS** or **Phone** as the type.
4. Enter the recipient phone number in E.164 format (e.g. `+12025551234`).
5. Repeat to add both SMS and phone if you want both channels.

## Step 6: Create Notification Rules

1. Still in **Profile & Settings**, go to **Notification Rules**.
2. Click **Add Rule**.
3. Select your SMS or phone contact method and set the delay:
   - `0` minutes: notify immediately when an alert fires
   - `5` minutes: notify after 5 minutes if the alert is still unacknowledged

Example escalation pattern:
- SMS at 0 minutes
- Phone call at 5 minutes

## Testing

1. Go to **Services** and select a service.
2. Under **Integration Keys**, click the **Test** button next to an integration key.
3. This fires a test alert through the full escalation path, including SMS/phone notifications.

Alternatively, send a test alert via the API:

```bash
curl -X POST "http://localhost:3000/api/v1/integrations/<your-secret>/alerts" \
  -H "Content-Type: application/json" \
  -d '{"summary": "Twilio test alert", "dedup_key": "twilio-test"}'
```

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `invalid phone number, use E.164 format` | Number missing `+` or country code | Use format `+12025551234` (no spaces, dashes, or parentheses) |
| `twilio SMS error 21608` | Twilio number doesn't have SMS capability | Buy a number with SMS enabled |
| `twilio call error 21215` | Geographic permission not enabled | In Twilio Console, go to **Voice > Settings > Geo Permissions** and enable the destination country |
| `twilio SMS error 21608` or `21610` | Trial account sending to unverified number | Verify the recipient under **Phone Numbers > Verified Caller IDs**, or upgrade your Twilio account |
| `twilio SMS error 21211` | Invalid `To` number | Check the recipient number exists and is in E.164 format |
| Notifications not sending | Twilio env vars not set or PageFire not restarted | Verify all three `PAGEFIRE_TWILIO_*` variables are set and restart PageFire |

Check PageFire logs (`PAGEFIRE_LOG_LEVEL=debug`) for detailed Twilio API error codes and messages.

## Cost Notes

Twilio is pay-as-you-go. Approximate US pricing:

| Channel | Cost |
|---|---|
| SMS (outbound, US) | ~$0.0079 per message |
| Phone call (outbound, US) | ~$0.014 per minute |
| Phone number (US local) | ~$1.15/month |

Calls use Twilio's text-to-speech (Alice voice) to read the alert message aloud, with the message repeated twice. A typical alert call lasts under 1 minute.

International rates vary. See [Twilio's pricing page](https://www.twilio.com/en-us/pricing) for current rates.
