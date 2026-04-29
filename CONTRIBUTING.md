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
```
### Project Architecture
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

### Branching Strategy
```bash
# Feature branches
git checkout -b feature/add-otp-login
```
```bash
# Bug fixes
git checkout -b fix/payment-timeout
```
```bash
# Documentation
git checkout -b docs/update-readme
```
```bash
# Performance
git checkout -b perf/cache-user-session
```
### Service Ports
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

### Branching Strategy
```bash
# Feature branches
git checkout -b feature/add-otp-login
```
```bash
# Bug fixes
git checkout -b fix/payment-timeout
```
# Documentation
git checkout -b docs/update-readme
```bash
# Performance
git checkout -b perf/cache-user-session
```
### Commit Convention
Types: feat, fix, docs, style, refactor, perf, test, chore

Scopes: auth, commercial, features, levels, dynasty, support, training, notifications, calendar, storage, financial, gateway, shared, proto, kong, scripts, docs

Examples: 
```bash
git commit -m "feat(auth): add SMS-based OTP login"
```
git commit -m "fix(commercial): handle Parsian payment timeout"
```bash
git commit -m "docs(contributing): add contribution guidelines"
```
git commit -m "perf(storage): implement FTP connection pooling"
```bash
git commit -m "test(auth): add unit tests for login service"
```
