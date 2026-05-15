# CHANGELOG
### [0.23.0] - 2026-05-16
---
#### Updated
- Update Vault Entry services API endpoints to work with Authentication middleware.

### [0.22.0] - 2026-05-15
---
#### Updated
- Update [PATCH] profile API service to update an existing user profile.

### [0.22.0] - 2026-05-15
---
#### Added
- Add Get User Details and Delete User functionality.

### [0.21.0] - 2026-05-15
---
#### Updated
- Update User Signup process to handle EncryptionSalt and EncryptionKey for vault entry security.

### [0.20.0] - 2026-05-14
---
#### Updated
- Update Users table schema.

### [0.21.0] - 2026-05-13
---
#### Updated
- Update Groups services API endpoints to work with Authentication middleware.

### [0.20.0] - 2026-05-13
---
#### Updated
- Update Groups table schema.

### [0.19.0] - 2026-05-13
---
#### Updated
- Update Profile services API endpoints to work with Authentication middleware.

### [0.18.1] - 2026-05-13
---
#### Updated
- Fix DB schema.
- Increase JWT validity to 24 hours.

### [0.18.0] - 2026-05-11
---
#### Added
- Add auth middleware for checking JWT validation on each protected API route.

### [0.17.0] - 2026-05-11
---
#### Added
- Add user login functionality.
- Add fetching environment variable support to the application.

### [0.16.0] - 2026-05-11
---
#### Added
- Add fetch user salt functionality.

### [0.15.0] - 2026-05-11
---
#### Updated
- Rename users table column `display_name` to `name`.

### [0.14.0] - 2026-05-08
---
#### Added
- Add API endpoint for the user sign up.

### [0.13.0] - 2026-04-10
---
#### Added
- Add Vault Entries' service API endpoints.

### [0.12.0] - 2026-04-10
---
#### Added
- Add Groups' service API endpoints

### [0.11.0] - 2026-04-10
---
#### Added
- Add Profiles' service API endpoints.

### [0.10.0] - 2026-04-10
---
#### Added
- Add DB models for type safety.

### [0.9.0] - 2026-04-09
---
#### Added
- Add Users' service API endpoints.

### [0.8.0] - 2026-04-09
---
#### Added
- Add DB migrations on server start-up.

### [0.7.0] - 2026-04-09
---
#### Added
- Initialize gin server to serve backend REST APIs.

### [0.6.0] - 2026-04-09
---
#### Added
- Add Database utility file to connect with a remote PostgreSQL DB instance.

### [0.5.0] - 2026-04-09
---
#### Added
- Add logger utility for better log management.

### [0.4.0] - 2026-04-09
---
#### Added
- Add configuration support using [viper](https://github.com/spf13/viper) for loading server config.

### [0.3.0] - 2026-03-10
---
#### Added
- Add `main` and `router` files for the server.

### [0.2.0] - 2026-03-10
---
#### Added
- Add support for [air](https://github.com/air-verse/air) for hot-reloading server.

### [0.1.0] - 2026-03-10
---
#### Added
- Initialize the repository.
