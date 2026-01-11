# ChainIndexor API Documentation

This directory contains the auto-generated Swagger/OpenAPI documentation for the ChainIndexor REST API.

## Files

- **swagger.json** - OpenAPI specification in JSON format
- **swagger.yaml** - OpenAPI specification in YAML format
- **docs.go** - Auto-generated Go code for embedding Swagger UI

## Accessing the API Documentation

When the ChainIndexor API server is running, you can access the interactive Swagger UI at:

```text
http://localhost:8080/swagger/index.html
```

This provides an interactive interface where you can:

- View all available endpoints
- See request/response schemas
- Test endpoints directly from the browser
- Generate client code

## API Endpoints

The API documentation covers all available endpoints including:

- **Health & Info**
  - `GET /health` - Health check
  - `GET /api/v1/indexers` - List all indexers

- **Event Queries**
  - `GET /api/v1/indexers/{name}/events` - Query events
  - `GET /api/v1/indexers/{name}/stats` - Get indexer statistics

- **Analytics**
  - `GET /api/v1/indexers/{name}/events/timeseries` - Time-series data
  - `GET /api/v1/indexers/{name}/metrics` - Performance metrics

## Updating Documentation

After modifying handler comments or types in the code, regenerate the documentation:

```bash
go run github.com/swaggo/swag/cmd/swag@latest init -g pkg/api/server.go --output ./pkg/api/docs
```

This will update:

- `swagger.json` - The OpenAPI specification
- `swagger.yaml` - YAML version of the specification
- `docs.go` - Go code for embedding the Swagger UI

## Integration with External Tools

The generated `swagger.json` can be imported into:

- **Postman** - For REST client testing
- **VS Code REST Client** - For endpoint development
- **Code generators** - To auto-generate SDKs for multiple languages
- **API Gateways** - For routing and rate limiting configuration

## Documentation Standards

All handler functions should include Swagger documentation comments:

```go
// GetEvents retrieves events from a specific indexer.
// @Summary Get events from an indexer
// @Description Retrieve events with optional filtering and pagination
// @Tags Events
// @Produce json
// @Param name path string true "Indexer name"
// @Param limit query int false "Maximum results" default(100)
// @Success 200 {object} EventResponse "List of events"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Router /indexers/{name}/events [get]
func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) { ... }
```

### Common Swagger Annotations

- `@Summary` - Brief description
- `@Description` - Detailed description
- `@Tags` - Endpoint category
- `@Param` - Query/path parameters
- `@Success` - Successful response
- `@Failure` - Error responses
- `@Router` - HTTP method and path
- `@Produce` - Response content types

For more information, visit the [Swaggo documentation](https://github.com/swaggo/swag).
