# API Usage

## Ingest PII Counts

**Endpoint**: `POST /ingest`

**Payload**:

```json
{
  "timestamp": "2026-04-23T15:34:00Z",
  "source_id": "application-server-1",
  "pii_types": {
    "credit-card": 50,
    "email": 120,
    "ssn": 5
  }
}
```

**Example Request**:

```bash
curl -k -X POST https://localhost:8443/ingest \
     -H "Content-Type: application/json" \
     -d '{
           "timestamp": "2026-04-23T15:34:00Z",
           "source_id": "test-source",
           "pii_types": {"credit-card": 10}
         }'
```
