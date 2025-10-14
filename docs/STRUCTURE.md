# Gin Web Server Structure Guide

## Directory Structure

```
.
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── api/                       # Feature-specific packages
│   │   └── ingestion/            # Example feature: ingestion
│   │       ├── controller.go     # HTTP handlers
│   │       ├── router.go         # Route definitions
│   │       ├── service.go        # Business logic
│   │       ├── types.go          # Request/Response DTOs
│   │       └── schema.go         # Database models
│   ├── config/
│   │   └── config.go             # Configuration management
│   ├── controllers/              # Shared/global controllers
│   │   ├── health_controller.go
│   │   └── system_controller.go
│   ├── loaders/
│   │   └── postgres.go           # Database client
│   ├── middleware/
│   │   ├── cors.go               # CORS middleware
│   │   └── request_id.go         # Request ID middleware
│   ├── routes/
│   │   ├── routes.go             # Main route setup
│   │   ├── api.go                # API routes
│   │   └── health.go             # Health check routes
│   └── utils/
│       ├── constants.go
│       └── logger.go
├── go.mod
├── go.sum
└── README.md
```

## Architecture Pattern

This structure follows a **feature-based modular architecture** with clear separation of concerns:

### 1. **Routes** (`internal/routes/`)
- Define all routes and their mappings
- Apply middleware
- Group related routes
- Keep routes organized by domain/feature

### 2. **Controllers** (`internal/controllers/` or `internal/api/*/controller.go`)
- Handle HTTP requests and responses
- Validate request data (using Gin binding)
- Call service layer
- Format responses
- Handle HTTP-specific concerns (status codes, headers)

### 3. **Services** (`internal/api/*/service.go`)
- Contain business logic
- Orchestrate operations
- Handle transactions
- Call repositories/databases
- Independent of HTTP layer

### 4. **Types/DTOs** (`internal/api/*/types.go`)
- Request DTOs (with validation tags)
- Response DTOs
- Error responses
- Clean separation from database models

### 5. **Schema/Models** (`internal/api/*/schema.go`)
- Database models
- Schema definitions
- Constants related to the feature

### 6. **Middleware** (`internal/middleware/`)
- Reusable middleware functions
- CORS, authentication, logging, etc.

## Adding a New Feature

To add a new feature (e.g., "users"), follow these steps:

### 1. Create feature directory structure:
```bash
mkdir -p internal/api/users
```

### 2. Create the files:

**`internal/api/users/types.go`** - Request/Response types
```go
package users

type CreateUserRequest struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

type UserResponse struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

**`internal/api/users/schema.go`** - Database models
```go
package users

type User struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}
```

**`internal/api/users/service.go`** - Business logic
```go
package users

type Service struct {
    db *loaders.PostgresClient
}

func NewService(db *loaders.PostgresClient) *Service {
    return &Service{db: db}
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*UserResponse, error) {
    // Business logic here
    return &UserResponse{}, nil
}
```

**`internal/api/users/controller.go`** - HTTP handlers
```go
package users

type Controller struct {
    service *Service
}

func NewController(service *Service) *Controller {
    return &Controller{service: service}
}

func (c *Controller) CreateUser(ctx *gin.Context) {
    var req CreateUserRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {
        ctx.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    response, err := c.service.CreateUser(ctx.Request.Context(), req)
    if err != nil {
        ctx.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    ctx.JSON(200, response)
}
```

**`internal/api/users/router.go`** - Route registration
```go
package users

func RegisterRoutes(router *gin.RouterGroup, db *loaders.PostgresClient) {
    service := NewService(db)
    controller := NewController(service)
    
    router.POST("/users", controller.CreateUser)
    router.GET("/users/:id", controller.GetUser)
    router.PUT("/users/:id", controller.UpdateUser)
    router.DELETE("/users/:id", controller.DeleteUser)
}
```

### 3. Register routes in `internal/routes/api.go`:
```go
import "github.com/LEVIII007/go-web-server-template/internal/api/users"

func SetupAPIRoutes(router *gin.Engine, db *loaders.PostgresClient, cfg *config.Config) {
    v1 := router.Group("/api/v1")
    {
        // ... existing routes ...
        
        // Register user routes
        users.RegisterRoutes(v1, db)
    }
}
```

## Benefits of This Structure

1. **Separation of Concerns**: Each layer has a specific responsibility
2. **Testability**: Easy to test each layer independently
3. **Scalability**: Add new features without affecting existing ones
4. **Maintainability**: Clear organization makes code easy to find and modify
5. **Reusability**: Shared components (middleware, utils) can be reused
6. **Clean Architecture**: Business logic is independent of HTTP layer

## Running the Application

```bash
# Run the application
go run cmd/main.go

# Build the application
go build -o bin/app cmd/main.go

# Run with environment variables
ENV=production go run cmd/main.go
```

## API Endpoints

### Health Checks
- `GET /health` - Full health check (includes database)
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe

### System Info
- `GET /api/v1/status` - System status
- `GET /api/v1/info` - System information

### Example: Ingestion (if enabled)
- `POST /api/v1/ingest` - Ingest single item
- `POST /api/v1/ingest/bulk` - Bulk ingest
- `GET /api/v1/ingest/:id` - Get ingestion by ID

## Best Practices

1. **Always use DTOs**: Never expose database models directly in API responses
2. **Validate at controller level**: Use Gin's binding and validation tags
3. **Keep controllers thin**: Move logic to services
4. **Use context**: Pass context for cancellation and timeouts
5. **Log appropriately**: Use structured logging (zap)
6. **Handle errors gracefully**: Return meaningful error messages
7. **Use middleware**: For cross-cutting concerns (CORS, auth, logging)
