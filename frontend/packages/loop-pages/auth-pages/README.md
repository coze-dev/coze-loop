# @cozeloop/auth-pages

Authentication and account management pages for CozeLoop.

## Overview

This package provides authentication-related pages and components for the CozeLoop platform, including login pages, account settings, and user management interfaces. It's part of the CozeLoop monorepo and integrates with the platform's authentication system.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/auth-pages": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

#### Using the Auth App (Default Export)

```typescript
import AuthApp from '@cozeloop/auth-pages';
import { BrowserRouter } from 'react-router-dom';

function App() {
  return (
    <BrowserRouter>
      <AuthApp />
    </BrowserRouter>
  );
}
```

The default export provides a complete auth application with routing:

- `/login` - Login page
- Automatic redirect to login for unmatched routes

#### Using Individual Components

```typescript
import { AccountSetting, SwitchLang } from '@cozeloop/auth-pages';

function SettingsPage() {
  return (
    <div>
      <SwitchLang />
      <AccountSetting />
    </div>
  );
}
```

## Features

- **Login Flow**: Complete login page with authentication handling
- **Account Settings**: User profile and account management interface
- **Personal Access Token (PAT) Management**: Create, view, and manage API tokens
- **User Information Panel**: Edit user profile, username, and account details
- **Language Switching**: Multi-language support with language switcher component
- **Responsive Auth Frame**: Consistent layout for authentication pages

## Components

### Main Exports

- **`App` (default)**: Complete auth application with routing
- **`AccountSetting`**: Account settings and management interface
- **`SwitchLang`**: Language switcher component

### Internal Components

- `AuthFrame`: Layout wrapper for auth pages
- `LoginPanel`: Login form component
- `Logo`: CozeLoop logo component
- `UserInfoPanel`: User profile management
- `PATPanel`: Personal Access Token management

## API Reference

### Default Export: `App`

A React component that provides the complete authentication application with built-in routing.

### Named Exports

#### `AccountSetting`

Component for managing user account settings, including:

- User information editing
- Personal Access Token (PAT) management
- Account preferences

#### `SwitchLang`

Language switcher component for changing the application language.

For detailed API documentation, please refer to the TypeScript definitions.

## Dependencies

This package depends on:

- `@cozeloop/account`: Account management functionality
- `@cozeloop/api-schema`: API type definitions
- `@cozeloop/i18n-adapter`: Internationalization support
- `@cozeloop/stores`: State management
- `@coze-arch/coze-design`: UI component library
- `react-router-dom`: Routing functionality

## Development

This package is built with:

- TypeScript for type safety
- React 18+ for UI components
- React Router for navigation
- Vitest for testing
- ESLint for code quality
- Coze Design System for UI components

### Scripts

```bash
# Build the package
npm run build

# Run linting
npm run lint

# Run tests
npm run test

# Run tests with coverage
npm run test:cov
```

## Contributing

This package is part of the CozeLoop monorepo. Please follow the monorepo contribution guidelines.

## License

Apache-2.0
