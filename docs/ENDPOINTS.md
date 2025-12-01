# PII Service API Endpoints

This document describes all available endpoints for the PII (Personally Identifiable Information) Service API.

## Base URL
```
http://{ServiceHost}:{APIPort}
```

## Authentication
All endpoints require proper organization credentials and client identification.

## Endpoints

### Health Check

#### GET /health
Check the health status of the API service and its dependencies.

**Response:**
```json
{
  "status": "healthy",
  "serviceName": "pii-service",
  "version": "1.0.0",
  "timestamp": "2025-11-28T10:30:00Z",
  "details": {
    "database": "healthy",
    "grpc_services": "healthy"
  }
}
```

---

### Tokenization

#### POST /v1/tokenize
Tokenize sensitive data by replacing it with a secure reference hash.

**Request Body:**
```json
{
  "data": "sensitive@email.com",
  "dataType": "email",
  "retentionPolicy": "gdpr-compliant",
  "clientId": "client-123",
  "organizationId": "acme-corp",
  "organizationKey": "super-secret-key",
  "metadata": {
    "source": "web-form",
    "user_id": "12345"
  }
}
```

**Parameters:**
- `data` (string, required): The sensitive data to tokenize
- `dataType` (string, required): Type of data (e.g., "email", "phone", "ssn")
- `retentionPolicy` (string, optional): Data retention policy
- `clientId` (string, required): Client identifier
- `organizationId` (string, required): Organization identifier
- `organizationKey` (string, required): Organization encryption key
- `metadata` (object, optional): Additional metadata as key-value pairs

**Success Response (200):**
```json
{
  "referenceHash": "tok_475c0f68cebc109e561dc3df093939c7",
  "tokenType": "sha256",
  "expiresAt": "2026-11-28T10:30:00Z",
  "status": "success"
}
```

**Error Response (400):**
```json
{
  "error": "error",
  "code": "TOKENIZE_ERROR",
  "message": "Invalid data format"
}
```

---

### Detokenization

#### POST /v1/detokenize
Retrieve original data using a reference hash.

**Request Body:**
```json
{
  "referenceHash": "tok_475c0f68cebc109e561dc3df093939c7",
  "purpose": "customer-service",
  "requestingService": "crm",
  "requestingUser": "user@example.com",
  "organizationId": "acme-corp",
  "organizationKey": "super-secret-key"
}
```

**Parameters:**
- `referenceHash` (string, required): The token reference hash
- `purpose` (string, required): Purpose for accessing the data
- `requestingService` (string, required): Service requesting the data
- `requestingUser` (string, required): User requesting the data
- `organizationId` (string, required): Organization identifier
- `organizationKey` (string, required): Organization encryption key

**Success Response (200):**
```json
{
  "data": "sensitive@email.com",
  "dataType": "email",
  "originalTimestamp": "2025-11-28T10:30:00Z",
  "accessLogged": true,
  "status": "success"
}
```

**Error Response (400):**
```json
{
  "error": "error",
  "code": "DETOKENIZE_ERROR",
  "message": "Token not found or expired"
}
```

---

### Audit Logs

#### GET /v1/audit/logs
Retrieve audit logs with optional filtering and pagination.

**Query Parameters:**
- `startTime` (string, optional): Start time filter (RFC3339 format)
- `endTime` (string, optional): End time filter (RFC3339 format)
- `referenceHash` (string, optional): Filter by specific token
- `requestingService` (string, optional): Filter by requesting service
- `limit` (integer, optional): Maximum number of results (default: unlimited)
- `offset` (integer, optional): Pagination offset (default: 0)

**Example Request:**
```
GET /v1/audit/logs?startTime=2025-11-28T00:00:00Z&limit=50&requestingService=crm
```

**Success Response (200):**
```json
{
  "logs": [
    {
      "auditId": "audit_tokenize_1732787400_123456789",
      "referenceHash": "tok_475c0f68cebc109e561dc3df093939c7",
      "operation": "tokenize",
      "requestingService": "api-gateway",
      "requestingUser": "client-123",
      "purpose": "gdpr-compliant",
      "timestamp": "2025-11-28T10:30:00Z",
      "clientIp": "192.168.1.100",
      "metadata": {
        "source": "web-form",
        "user_id": "12345"
      }
    }
  ],
  "totalCount": 1,
  "status": "success"
}
```

---

### Metrics

#### GET /v1/metrics
Retrieve Prometheus metrics for monitoring and observability.

**Response:**
Returns Prometheus-formatted metrics including:
- Request counts by endpoint and status
- Request duration histograms
- Tokenization/detokenization counters
- Service health metrics

**Example Response:**
```
# HELP api_requests_total Total number of API requests
# TYPE api_requests_total counter
api_requests_total{endpoint="/v1/tokenize",method="POST",status="200"} 42
api_requests_total{endpoint="/v1/detokenize",method="POST",status="200"} 38

# HELP api_request_duration_seconds Duration of API requests
# TYPE api_request_duration_seconds histogram
api_request_duration_seconds_bucket{endpoint="/v1/tokenize",method="POST",le="0.005"} 35
...
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:

- `200` - Success
- `400` - Bad Request (validation errors)
- `500` - Internal Server Error

Application-level errors include structured error responses with `error`, `code`, and `message` fields.

## Security Features

- **Audit Logging**: All operations are automatically logged for compliance
- **Encryption**: Data is encrypted at rest and in transit
- **Access Control**: Organization-based isolation
- **Token Expiration**: Configurable token lifetimes
- **Rate Limiting**: Built-in request throttling

## Data Types

Supported data types for tokenization:
- `email` - Email addresses
- `phone` - Phone numbers
- `ssn` - Social Security Numbers
- `credit_card` - Credit card numbers
- `name` - Personal names
- `address` - Physical addresses

## Rate Limiting

The API implements rate limiting based on:
- Organization ID
- Client ID
- IP address

## Monitoring

All endpoints are instrumented with Prometheus metrics for:
- Request latency
- Error rates
- Throughput monitoring
- Service health checks

## Examples

See the `examples/` directory for complete JavaScript examples of API usage.