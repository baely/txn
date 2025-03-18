# Is James in the Office

A standalone service that monitors Monzo transactions to detect when James is in the office based on coffee purchases near the workplace.

## What it does

This service monitors Monzo banking transactions to:

- **Office Presence Tracking**: Automatically detects when James has purchased coffee nearby, indicating he is in the office
- **Real-time Notifications**: Sends updates to Slack when status changes

This service is accessible through the domain `isjamesintheoffice.today` and provides a simple yes/no interface.

## Getting Started

### Requirements

- Go 1.23+
- Monzo account with API access
- Webhook endpoint for receiving transaction notifications

### Setup

1. Set up environment variables:
   ```bash
   export MONZO_ACCESS_TOKEN=your_token
   export MONZO_WEBHOOK_SECRET=your_secret
   export SLACK_WEBHOOK=your_webhook_url
   ```

2. Run locally or with Docker:
   ```bash
   # Local
   go run main.go

   # Docker
   docker-compose up -d
   ```

## Configuration

Set these environment variables:

| Variable | Description |
|----------|-------------|
| `MONZO_ACCESS_TOKEN` | Monzo Banking API token |
| `MONZO_WEBHOOK_SECRET` | Webhook validation secret |
| `SLACK_WEBHOOK` | Slack notification URL |