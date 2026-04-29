# Contributing to MetaRGB Microservices

First off, thank you for considering contributing to MetaRGB Microservices. This document provides guidelines and instructions for contributing to this project.

## Table of Contents
1. [Code of Conduct](#code-of-conduct)
2. [Key Principles](#key-principles)
3. [Getting Started](#getting-started)
4. [Project Architecture](#project-architecture)
5. [Development Workflow](#development-workflow)
6. [Commit Convention](#commit-convention)
7. [Writing Code](#writing-code)
8. [Testing](#testing)
9. [Pull Request Process](#pull-request-process)
10. [Reporting Bugs](#reporting-bugs)
11. [Coding Standards](#coding-standards)
12. [API Compatibility (CRITICAL)](#api-compatibility-critical)
13. [Getting Help](#getting-help)

## Code of Conduct

This project adheres to a Code of Conduct. By participating, you are expected to uphold this code. Report unacceptable behavior to the project maintainers.

## Key Principles

- **100% API Compatibility**: All services MUST maintain 100% API compatibility with the original Laravel monolith – JSON fields, status codes, validation formats, Jalali dates, and URL structures.
- **Security First**: Never hardcode secrets. Use `config.env` files only.
- **Test Coverage**: Every change must be covered by unit, integration, and golden tests.
- **Layered Architecture**: Strictly follow `handler → service → repository` pattern.
- **Environment Parity**: Development must work with Docker Compose.

## Getting Started

### Prerequisites

| Tool | Version | Command |
|------|---------|---------|
| Go | 1.21+ | `go version` |
| Protocol Buffers | latest | `protoc --version` |
| Docker & Docker Compose | latest | `docker --version` |
| Node.js | 18+ | `node --version` |
| Make | latest | `make --version` |

### Setup Development Environment

```bash
# 1. Fork and clone
git clone https://github.com/YOUR_USERNAME/microservice-metarang.git
cd microservice-metarang

# 2. Add upstream
git remote add upstream https://github.com/iranpsc/microservice-metarang.git

# 3. Create config files for your service
cp services/auth-service/config.env.sample services/auth-service/config.env
# Edit config.env with your values

# 4. Generate proto files
make proto

# 5. Start infrastructure
docker compose up -d mysql redis
sleep 10
make import-schema

# 6. Run development environment
make dev

# 7. Verify
make ps
curl http://localhost:8000
curl http://localhost:3002/health
Project Architecture
text
metargb-microservices/
├── services/                     # Each microservice
│   ├── auth-service/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── handler/          # gRPC handlers
│   │   │   ├── service/          # Business logic
│   │   │   └── repository/       # Data access
│   │   └── config.env.sample
│   ├── commercial-service/
│   ├── features-service/
│   ├── levels-service/
│   ├── dynasty-service/
│   ├── support-service/
│   ├── training-service/
│   ├── notifications-service/
│   ├── calendar-service/
│   ├── storage-service/
│   ├── financial-service/
│   ├── grpc-gateway/
│   └── websocket-gateway/
├── shared/
│   ├── proto/                    # .proto definitions
│   ├── pb/                       # Generated code
│   └── pkg/                      # Shared packages (db, auth, logger, metrics)
├── kong/                         # Kong Gateway config
├── scripts/                      # Database schema
├── tests/                        # Integration & golden tests
├── docs/                         # Documentation
├── monitoring/                   # Grafana dashboards
├── k8s/                          # Kubernetes manifests
├── .cursor/rules/                # LLM assistant rules
├── Makefile
└── docker-compose.yml
Service Ports
Service	gRPC Port	HTTP Port
auth-service	50051	-
commercial-service	50052	-
features-service	50053	-
levels-service	50054	-
dynasty-service	50055	-
support-service	50056	-
training-service	50057	-
notifications-service	50058	-
calendar-service	50059	-
storage-service	50060	8059
financial-service	50062	-
grpc-gateway	-	8080
websocket-gateway	-	3002
Kong Gateway	-	8000
Development Workflow
Branching Strategy
bash
# Feature branches
git checkout -b feature/add-otp-login

# Bug fixes
git checkout -b fix/payment-timeout

# Documentation
git checkout -b docs/update-readme

# Performance
git checkout -b perf/cache-user-session
Commit Convention
Follow Conventional Commits:

text
<type>(<scope>): <subject>
Types: feat, fix, docs, style, refactor, perf, test, chore

Scopes: auth, commercial, features, levels, dynasty, support, training, notifications, calendar, storage, financial, gateway, shared, proto, kong, scripts, docs

Examples:

bash
git commit -m "feat(auth): add SMS-based OTP login"
git commit -m "fix(commercial): handle Parsian payment timeout"
git commit -m "docs(contributing): add contribution guidelines"
git commit -m "perf(storage): implement FTP connection pooling"
git commit -m "test(auth): add unit tests for login service"
Writing Code
Layered Architecture (MANDATORY)
Layer	Responsibility	Depends On
handler	gRPC request/response, error mapping	service
service	Business logic	repository
repository	Database operations	models
Code Example
go
// auth-service/internal/handler/auth.go
package handler

import (
    "context"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    pb "github.com/iranpsc/microservice-metarang/shared/pb/auth"
)

type AuthHandler struct {
    pb.UnimplementedAuthServiceServer
    authService *service.AuthService
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
    token, err := h.authService.Login(ctx, req.Email, req.Password)
    if err != nil {
        return nil, status.Error(codes.Unauthenticated, err.Error())
    }
    return &pb.LoginResponse{Token: token}, nil
}
go
// auth-service/internal/service/auth.go
package service

type AuthService struct {
    userRepo repository.UserRepository
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
    user, err := s.userRepo.FindByEmail(ctx, email)
    if err != nil {
        return "", err
    }
    
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        return "", errors.New("invalid credentials")
    }
    
    return generateJWT(user), nil
}
Shared Packages
Always use packages from shared/pkg:

db – MySQL connection with reconnect

auth – JWT and permission helpers

logger – Structured logging (JSON in prod, text in dev)

metrics – Prometheus metrics

helpers – Jalali date conversion, validation

Configuration Management
go
// Inside cmd/server/main.go
dbHost := os.Getenv("DB_HOST")  // Comes from config.env
Never hardcode environment variables. Use config.env.sample as template.

Testing
Run Tests
bash
make test-unit        # Unit tests (fast)
make test-integration # Integration tests (requires Docker)
make test-golden      # Golden tests - compares with Laravel monolith
make test-database    # Database tests
make test-all         # Complete test suite
Unit Test Example
go
// auth-service/internal/service/auth_test.go
func TestAuthService_Login_Success(t *testing.T) {
    mockRepo := new(MockUserRepository)
    mockRepo.On("FindByEmail", "test@example.com").Return(&models.User{
        Email:    "test@example.com",
        Password: "$2a$10$N9qo8uLOickgx2ZMRZoMy.MrqDkfGj5Z5Rj5G5Rj5G5Rj5G5Rj5G",
    }, nil)

    svc := NewAuthService(mockRepo)
    token, err := svc.Login(context.Background(), "test@example.com", "password123")

    assert.NoError(t, err)
    assert.NotEmpty(t, token)
    mockRepo.AssertExpectations(t)
}
Golden Tests (CRITICAL)
Golden tests ensure 100% compatibility with Laravel monolith. If a golden test fails, your PR will be rejected.

To update golden files (only with maintainer approval):

bash
make test-golden -update
Never update golden files without explicit approval.

Pull Request Process
Before Submitting PR
bash
# 1. Sync with upstream
git checkout main
git pull upstream main
git checkout your-branch
git rebase main

# 2. Run tests
make test-all

# 3. Run linter
golangci-lint run

# 4. Push
git push origin your-branch
PR Template
markdown
## Changes Made
- Clear description of changes

## Change Type
- [ ] feat (new feature)
- [ ] fix (bug fix)
- [ ] docs (documentation)
- [ ] refactor (code rewrite)
- [ ] test (tests)
- [ ] chore (build/tooling)

## How Tested
- [ ] Unit tests
- [ ] Integration tests
- [ ] Golden tests
- [ ] Manual testing

## API Compatibility with Laravel Monolith
- [ ] Yes, 100% compatible
- [ ] No (explain why)

## Self-Review Checklist
- [ ] Code passes linting
- [ ] Commit messages follow conventional commits
- [ ] Documentation updated
- [ ] No secrets in code
- [ ] Tests written for new features
- [ ] `make test-all` passes locally
After Submitting PR
At least one maintainer will review your PR.

Address feedback promptly.

Once approved, a maintainer will merge.

Reporting Bugs
Use GitHub Issues with the Bug Report template:

markdown
**Describe the bug**
Clear description.

**To Reproduce**
Steps to reproduce.

**Expected behavior**
What should happen.

**Actual behavior**
What actually happens with logs.

**Environment**
- Service: [auth-service, commercial-service, etc.]
- Go version:
- Docker version:
- OS:

**Additional context**
Coding Standards
Go Standards
Follow Effective Go

Use gofmt and goimports

Maximum line length: 120 characters

Handle all errors explicitly – no _ ignoring errors

Use meaningful variable names

Project-Specific Rules
Layered architecture strictly enforced

Logging: Use shared/pkg/logger – never fmt.Println

Context propagation: Always pass context.Context as first parameter

Dependency injection: Use constructors, no global state

Error wrapping: Use fmt.Errorf("...: %w", err)

Proto Guidelines
Use snake_case for field names

Use PascalCase for message names

Add comments for all fields

Version your proto files (v1, v2)

API Compatibility (CRITICAL)
This is the most important rule of this project.

All services MUST maintain 100% API compatibility with the Laravel monolith:

JSON field names – exact same casing and spelling

Status codes – same HTTP/gRPC status codes

Validation formats – same error message structure

Jalali dates – same format (e.g., "1402-01-15")

URL structures – same endpoints and parameters

Compatibility Testing
bash
# Run golden tests before every PR
make test-golden

# If this fails, find what changed:
# 1. Compare with Laravel monolith output
# 2. Fix your code to match exactly
# 3. Do NOT update golden files without approval
Breaking Changes
If you MUST make a breaking change (rare):

Open an issue first explaining why

Get written approval from maintainers

Update documentation

Create migration guide

Only then update golden files with make test-golden -update

Local Development Without Docker
bash
# 1. Start MySQL and Redis locally
# 2. Import schema
mysql -u root -p metargb_db < scripts/schema.sql

# 3. Configure service
cd services/auth-service
cp config.env.sample config.env
# Edit config.env: set DB_HOST=localhost

# 4. Run service
go run cmd/server/main.go

# For WebSocket gateway
cd websocket-gateway
npm install
npm start
Common Commands
bash
make dev              # Start full dev environment
make ps               # Check service status
make logs             # View all logs
make logs-service SERVICE=auth-service  # Service-specific logs
make down             # Stop all services
make build-all        # Build all images
make restart-service SERVICE=auth-service
make clean            # Stop and remove volumes
make kong-validate    # Validate Kong config
make kong-reload      # Reload Kong
make clean-proto && make proto  # Regenerate proto files
Troubleshooting
Issue	Solution
Services not starting	docker compose logs auth-service
Database connection	docker exec metargb-mysql mysql -uroot -proot_password -e "SELECT 1"
Port already in use	lsof -i :50051 (macOS) or netstat -tulpn | grep 50051 (Linux)
Proto generation errors	make clean-proto && make proto
Golden tests fail	Compare with Laravel output, check date formats
415 Unsupported Media Type	Use gRPC client or port 8080 for REST
Reset everything	make clean && make dev
Getting Help
Documentation: Check docs/ folder

Issues: Search existing issues on GitHub

Discussions: Use GitHub Discussions

Email: Contact project maintainers

License
By contributing, you agree that your contributions will be licensed under the project's proprietary license.

Thank you for contributing to MetaRGB Microservices! 🚀
