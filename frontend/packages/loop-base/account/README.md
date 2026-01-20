# @cozeloop/account

CozeLoop account

## Overview

This package is part of the Coze Loop monorepo and provides authentication functionality. It includes hook, store, service.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/account": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import { /* exported functions/components */ } from '@cozeloop/account';

// Example usage
// TODO: Add specific usage examples
```

## Features

- Hook
- Store
- Service

## API Reference

### Exports

- `useUserStore, setUserInfo`
- `useSpaceStore,
  setSpace,
  PERSONAL_ENTERPRISE_ID,`
- `useLogin`
- `useRegister`
- `useLoginStatus`
- `useLogout`
- `useCheckLogin`
- `userService`
- `authnService`
- `spaceService`


For detailed API documentation, please refer to the TypeScript definitions.

## Development

This package is built with:

- TypeScript
- Modern JavaScript
- Vitest for testing
- ESLint for code quality

## Contributing

This package is part of the Coze Loop monorepo. Please follow the monorepo contribution guidelines.

## License

Apache-2.0
