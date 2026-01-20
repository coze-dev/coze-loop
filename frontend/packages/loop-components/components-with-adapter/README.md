# @cozeloop/components

common components for cozeloop

## Overview

This package is part of the Coze Loop monorepo and provides ui component functionality. It serves as a core component in the Coze Loop ecosystem.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/components": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import { /* exported functions/components */ } from '@cozeloop/components';

// Example usage
// TODO: Add specific usage examples
```

## Features

- Core functionality for Coze Loop
- TypeScript support
- Modern ES modules

## API Reference

### Exports

- `ColumnSelector, type ColumnItem`
- `TooltipWhenDisabled`
- `LoopTable`
- `TableWithPagination,
  DEFAULT_PAGE_SIZE,
  PAGE_SIZE_OPTIONS,`
- `PageError,
  PageLoading,
  PageNoAuth,
  PageNoContent,
  PageNotFound,`
- `TableColActions`
- `LoopTabs`
- `LargeTxtRender`
- `InputSlider`
- `handleCopy, sleep`

*And more...*

For detailed API documentation, please refer to the TypeScript definitions.

## Development

This package is built with:

- TypeScript
- React
- Vitest for testing
- ESLint for code quality

## Contributing

This package is part of the Coze Loop monorepo. Please follow the monorepo contribution guidelines.

## License

Apache-2.0
