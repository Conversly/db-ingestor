myapp/
├── go.mod
├── README.md
├── cmd/
│    └── main.go                  # Entry point
├── internal/
│   ├── api/
│   │   ├── your-feature/
│   │   │   ├── handler.go           # alpha-controller.ts
│   │   │   ├── helper.go            # alpha-helper.ts
│   │   │   ├── router.go            # alpha-router.ts
│   │   │   ├── schema.go            # alpha-schema.ts
│   │   │   └── service.go           # alpha-service.ts
│   │   └── index.go                  # Registers all feature routers to main router
│   ├── config/
│   │   └── config.go                 # App configuration, env vars
│   ├── queries/
│   │   ├── user_queries.go
│   │   └── order_queries.go
│   ├── loaders/
│   │   └── postgres.go
│   │   └── auth_loader.go
│   ├── shared/
│   │   ├── constants.go
│   │   ├── utils.go
│   │   ├── middleware.go            # Logging, auth, recovery, rate limiting
│   │   ├── jwt.go
│   └── utils/
│       ├── api_error.go
│       ├── catch_async.go
│       ├── request.go
│       ├── logger.go
│       └── validator.go