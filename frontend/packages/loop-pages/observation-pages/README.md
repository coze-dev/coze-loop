# @cozeloop/observation-pages

Observability and tracing pages for CozeLoop platform, providing comprehensive trace monitoring, analysis, and debugging capabilities.

## Overview

This package provides the observability system pages for the CozeLoop platform. It offers trace monitoring and analysis capabilities, allowing users to track, search, and debug application execution flows. The package integrates with the CozeLoop observation infrastructure to provide detailed insights into span execution, performance metrics, and trace data.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/observation-pages": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import ObservationApp from '@cozeloop/observation-pages';
import { BrowserRouter } from 'react-router-dom';

function App() {
  return (
    <BrowserRouter>
      <ObservationApp />
    </BrowserRouter>
  );
}
```

The app provides comprehensive trace monitoring and analysis with automatic routing configuration.

## Features

### Trace Monitoring

- **Trace List View**: Browse and search all traces with advanced filtering
- **Trace Detail Panel**: Deep dive into individual trace execution flows
- **Span Analysis**: Examine individual spans with detailed metrics
- **Real-time Search**: Search traces by various criteria including trace ID, status, and custom fields
- **Performance Metrics**: Track latency, tokens, and other performance indicators
- **Multi-language Support**: Internationalization with Chinese and English locales

### Key Capabilities

- **Advanced Filtering**: Filter traces by platform type, date range, status, and custom fields
- **Column Customization**: Configure visible columns including:
  - Status and trace/span identifiers
  - Input/output data
  - Token usage (input, output, total)
  - Latency metrics (total and first response)
  - Timestamps and metadata
  - Prompt keys and span information

- **Trace Details**: View comprehensive trace information including:
  - Span hierarchy and relationships
  - Advanced trace information
  - Input/output data for each span
  - Performance metrics and timing data

- **Custom Views**: User-configurable views with customizable columns and filters

## Routes

The application provides the following routes:

- `/` - Redirects to traces page
- `/traces` - Main trace monitoring and analysis page

## API Reference

### Default Export

The package exports a single React component that provides the complete observation application with built-in routing.

```typescript
import ObservationApp from '@cozeloop/observation-pages';
```

### Features Integration

The observation pages integrate with:

- **Trace List Component**: `CozeloopTraceListWithDetailPanel` from `@cozeloop/observation-components`
- **Field Metadata**: Dynamic field metadata fetching based on platform and span types
- **API Integration**: Direct integration with `observabilityTrace` API for data fetching
- **User Context**: Integration with workspace and user information

### Configurable Columns

The following columns are available for trace viewing:

- `status` - Trace execution status
- `trace_id` - Unique trace identifier
- `span_id` - Span identifier
- `span_type` - Type of span
- `span_name` - Name of the span
- `input` - Input data
- `output` - Output data
- `tokens` - Total token count
- `input_tokens` - Input token count
- `output_tokens` - Output token count
- `latency` - Total latency
- `latency_first_resp` - First response latency
- `start_time` - Execution start time
- `prompt_key` - Associated prompt key
- `logic_delete_date` - Logical deletion date

## Dependencies

This package integrates with:

- `@cozeloop/observation-components`: Core observation UI components and utilities
- `@cozeloop/api-schema`: API type definitions and observation schemas
- `@cozeloop/i18n-adapter`: Internationalization support
- `@cozeloop/components`: Shared UI components (PrimaryPage, etc.)
- `@cozeloop/biz-hooks-adapter`: Business logic hooks (workspace, user info)
- `@cozeloop/guard`: Access control and permissions
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
