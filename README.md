# TXN

TXN is a multi-service Go application that connects to banking APIs to monitor transaction activity. The application analyzes purchase patterns to provide useful insights and notifications.

## What it does

TXN monitors your banking transactions to:

- **Office Presence Tracking**: Automatically detects when someone has purchased coffee nearby, indicating they are in the office. This helps team members know when colleagues are available for in-person collaboration.
  
- **Caffeine Consumption Monitoring**: Tracks coffee and caffeinated beverage purchases over time, providing insights into consumption patterns and spending habits.
  
- **Real-time Notifications**: Sends updates to Slack when status changes, keeping team members informed without manual check-ins.

This application is particularly valuable for remote-friendly teams who want to coordinate office visits, track spending patterns on beverages, and maintain team awareness through automated notifications.

## Services

The app consists of the following services:

### Bailey's Services
- **Balance Service**: Receives webhooks from Up Banking
- **Presence Service**: Shows if Bailey is in the office based on coffee purchases
- **Tracker Service**: Records caffeine consumption data

Each service is exposed through its own domain:
- `events.baileys.dev` → Balance Service
- `isbaileybutlerintheoffice.today` → Presence Service
- `baileyneeds.coffee` → Tracker Service

### James's Services
- **Monzo Webhook Service**: Receives webhooks from Monzo Banking
- **James Presence Service**: Shows if James is in the office based on coffee purchases

Each service is exposed through its own domain:
- `events.james.dev` → Monzo Webhook Service
- `isjamesintheoffice.today` → James Presence Service

## Getting Started

### Requirements

- Go 1.23+
- PostgreSQL database (for Bailey's service)
- Up Banking or Monzo account with API access

### Bailey's Service Setup

1. Clone the repository
2. Copy environment template: `cp .env.example .env`
3. Update `.env` with your credentials
4. Run locally or with Docker:

```bash
# Local
go run main.go

# Docker
docker-compose up -d
```

### James's Service Setup

1. Set environment variables for Monzo API:
   ```bash
   export MONZO_ACCESS_TOKEN=your_token
   export MONZO_WEBHOOK_SECRET=your_secret
   export SLACK_WEBHOOK=your_webhook_url
   ```

2. Run locally or with Docker:
   ```bash
   # Local
   go run cmd/james/main.go

   # Docker (from cmd/james directory)
   docker-compose up -d
   ```

## Configuration

### Bailey's Service Configuration

| Variable | Description |
|----------|-------------|
| `UP_ACCESS_TOKEN` | Up Banking API token |
| `UP_WEBHOOK_SECRET` | Webhook validation secret |
| `SLACK_WEBHOOK` | Slack notification URL |
| `DB_USER` | PostgreSQL username |
| `DB_PASSWORD` | PostgreSQL password |
| `DB_HOST` | PostgreSQL hostname |
| `DB_PORT` | PostgreSQL port (default: 5432) |
| `DB_NAME` | PostgreSQL database name |

### James's Service Configuration

| Variable | Description |
|----------|-------------|
| `MONZO_ACCESS_TOKEN` | Monzo Banking API token |
| `MONZO_WEBHOOK_SECRET` | Webhook validation secret |
| `MONZO_WEBHOOK_URL` | Full URL where Monzo should send webhooks (e.g., `https://events.james.dev/event`) |
| `SLACK_WEBHOOK` | Slack notification URL |

## Project Structure

```
cmd/
  └── james/        # James in the Office standalone service
internal/
  ├── common/        # Shared utilities
  ├── balance/       # Up Banking webhook handler
  ├── ibbitot/       # Bailey's office presence tracker 
  ├── monzo/         # Monzo API integration and webhook handler
  ├── tracker/       # Transaction tracker
  └── server/        # HTTP server
```

## System Architecture

The following diagram shows how components interact within the system:

```mermaid
flowchart LR
    %% External Systems
    UpBanking[Up Banking API] -->|sends webhooks| BalanceService
    MonzoBanking[Monzo Banking API] -->|sends webhooks| MonzoWebhookService
    PostgreSQL[(PostgreSQL DB)] <-->|stores/retrieves events| TrackerService
    BaileyPresenceService -->|sends notifications| Slack1[Slack Webhook]
    JamesPresenceService -->|sends notifications| Slack2[Slack Webhook]
    
    %% Bailey Services
    subgraph BaileyMonolith[Bailey TXN Monolith]
      Router1[Chi Router\ninternal/server]
      BalanceService[Balance Service\ninternal/balance]
      BaileyPresenceService[Bailey Presence Service\ninternal/ibbitot]
      TrackerService[Tracker Service\ninternal/tracker]
      
      %% Internal routing
      Router1 -->|routes by domain| BalanceService
      Router1 -->|routes by domain| BaileyPresenceService
      Router1 -->|routes by domain| TrackerService
    end
    
    %% James Services
    subgraph JamesMonolith[James TXN Monolith]
      Router2[Chi Router\ninternal/server]
      MonzoWebhookService[Monzo Webhook Service\ninternal/monzo]
      JamesPresenceService[James Presence Service\ninternal/monzo]
      
      %% Internal routing
      Router2 -->|routes by domain| MonzoWebhookService
      Router2 -->|routes by domain| JamesPresenceService
    end
    
    %% Service Interactions
    BalanceService -->|dispatches events| BaileyPresenceService
    BalanceService -->|dispatches events| TrackerService
    BalanceService -->|fetches details| UpBanking
    
    MonzoWebhookService -->|dispatches events| JamesPresenceService
    MonzoWebhookService -->|fetches details| MonzoBanking
    
    %% Web Interfaces
    BaileyPresenceService -->|serves status page| WebUI1[Web UI\nisbaileybutlerintheoffice.today]
    TrackerService -->|displays consumption| WebUI2[Web UI\nbaileyneeds.coffee]
    JamesPresenceService -->|serves status page| WebUI3[Web UI\nisjamesintheoffice.today]
    
    %% External requests
    User((User)) -->|submits requests| Router1
    User -->|submits requests| Router2
    
    %% Styles
    classDef external fill:#f96,stroke:#333,stroke-width:2px
    classDef service fill:#58f,stroke:#333,stroke-width:2px
    classDef router fill:#5d8,stroke:#333,stroke-width:2px
    classDef monolith fill:#eee,stroke:#333,stroke-width:1px
    
    class UpBanking,MonzoBanking,PostgreSQL,Slack1,Slack2,User external
    class BalanceService,BaileyPresenceService,TrackerService,MonzoWebhookService,JamesPresenceService service
    class Router1,Router2 router
    class BaileyMonolith,JamesMonolith monolith
```

### Data Flow

#### Bailey's Service Flow
1. **Up Banking → Balance Service**: 
   - Up Banking sends transaction webhooks to the Balance Service
   - Balance Service validates and enriches transaction data

2. **Balance Service → Service Handlers**:
   - Distributes `TransactionEvent` to registered services
   - Each service filters relevant transactions

3. **Presence Service**:
   - Determines office presence based on coffee purchases
   - Updates web UI and sends Slack notifications

4. **Tracker Service**:
   - Records caffeine consumption in PostgreSQL
   - Calculates caffeine levels and provides visualization

#### James's Service Flow
1. **Monzo Banking → Monzo Webhook Service**: 
   - Monzo Banking sends transaction webhooks to the Monzo Webhook Service
   - Monzo Webhook Service validates and enriches transaction data

2. **Monzo Webhook Service → Service Handlers**:
   - Distributes `TransactionEvent` to registered services
   - Each service filters relevant transactions

3. **James Presence Service**:
   - Determines office presence based on coffee purchases
   - Updates web UI and sends Slack notifications

## Development

See [CLAUDE.md](CLAUDE.md) for development guidelines.
