# DataX Backend API

A Gin-based backend service for the DataX decentralized data network, built on Aptos blockchain.

## Features

- User initialization and data management
- Data submission and retrieval
- Access control (grant/revoke access)
- User vault management
- Token operations (register, mint)

## Prerequisites

- Go 1.21 or higher
- Aptos testnet account (for testing)

## Setup

1. Install dependencies:
```bash
go mod download
```

2. Copy environment file:
```bash
cp .env.example .env
```

3. Update `.env` with your configuration:
   - Set `DATAX_MODULE_ADDR` to your deployed module address
   - Set `NETWORK_MODULE_ADDR` to your deployed network module address
   - Adjust `APTOS_NODE_URL` if using a different network

4. Run the server:
```bash
go run main.go
```

The server will start on `http://localhost:8080` (or the port specified in `.env`).

## API Endpoints

### Health Check
- `GET /health` - Check if the service is running

### User Operations
- `POST /api/v1/users/initialize` - Initialize user's data store and vault
  ```json
  {
    "private_key": "0x..."
  }
  ```

### Data Operations
- `POST /api/v1/data/submit` - Submit data to the registry
  ```json
  {
    "private_key": "0x...",
    "data_hash": "hash_string",
    "metadata": "metadata_string"
  }
  ```

- `POST /api/v1/data/delete` - Delete a dataset
  ```json
  {
    "private_key": "0x...",
    "dataset_id": 0
  }
  ```

- `POST /api/v1/data/get` - Get dataset information
  ```json
  {
    "user": "0x...",
    "dataset_id": 0
  }
  ```

### Access Control
- `POST /api/v1/access/grant` - Grant access to a requester
  ```json
  {
    "private_key": "0x...",
    "dataset_id": 0,
    "requester": "0x...",
    "expires_at": 1234567890
  }
  ```

- `POST /api/v1/access/revoke` - Revoke access from a requester
  ```json
  {
    "private_key": "0x...",
    "dataset_id": 0,
    "requester": "0x..."
  }
  ```

- `POST /api/v1/access/check` - Check if a requester has access
  ```json
  {
    "owner": "0x...",
    "dataset_id": 0,
    "requester": "0x..."
  }
  ```

### Vault Operations
- `POST /api/v1/vault/get` - Get user's vault datasets
  ```json
  {
    "user": "0x..."
  }
  ```

### Token Operations
- `POST /api/v1/token/register` - Register to receive tokens
  ```json
  {
    "private_key": "0x..."
  }
  ```

- `POST /api/v1/token/mint` - Mint tokens to a recipient
  ```json
  {
    "private_key": "0x...",
    "recipient": "0x...",
    "amount": 1000
  }
  ```

## Response Format

All endpoints return a JSON response in the following format:

```json
{
  "success": true,
  "message": "Optional message",
  "data": { ... },
  "error": "Error message if success is false"
}
```

## Security Notes

⚠️ **Important**: This backend requires private keys in requests. In production:

1. Never expose private keys in API requests
2. Implement proper authentication and authorization
3. Use secure key management (e.g., hardware security modules)
4. Implement rate limiting
5. Use HTTPS only
6. Consider using wallet integration instead of passing private keys

## Development

### Project Structure

```
backend/
├── main.go              # Application entry point
├── config/              # Configuration management
├── models/              # Request/response models
├── handlers/            # HTTP handlers
├── services/            # Business logic and Aptos SDK integration
└── .env                 # Environment variables (not in git)
```

### Testing

To test the API endpoints, you can use `curl` or tools like Postman:

```bash
# Health check
curl http://localhost:8080/health

# Initialize user
curl -X POST http://localhost:8080/api/v1/users/initialize \
  -H "Content-Type: application/json" \
  -d '{"private_key": "0x..."}'
```

## License

MIT

