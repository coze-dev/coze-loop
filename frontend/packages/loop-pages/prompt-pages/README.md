# @cozeloop/prompt-pages

Prompt engineering and management pages for CozeLoop platform, providing comprehensive prompt development, testing, and lifecycle management.

## Overview

This package provides the prompt engineering (PE) system pages for the CozeLoop platform. It offers a complete workflow for creating, editing, testing, and managing prompts with integrated observability features. The package includes a prompt list view, development environment, and playground for experimentation.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/prompt-pages": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import PromptApp from '@cozeloop/prompt-pages';
import { BrowserRouter } from 'react-router-dom';

function App() {
  return (
    <BrowserRouter>
      <PromptApp />
    </BrowserRouter>
  );
}
```

The app provides comprehensive prompt engineering capabilities with automatic routing and lazy-loaded pages for optimal performance.

## Features

### Prompt List Management

- **Prompt Library**: Browse and manage all prompts in your workspace
- **Advanced Search**: Filter prompts by creator, name, tags, and other attributes
- **Batch Operations**: Create, edit, copy, and delete prompts
- **Permission Control**: Owner-based deletion permissions
- **Quick Access**: Click to edit, Shift/Cmd/Ctrl+Click to open in new window
- **Call Records**: Direct access to prompt execution traces via observation system

### Prompt Development

- **Interactive Editor**: Full-featured prompt development environment
- **Version Management**: Track and manage prompt versions
- **Model Integration**: Support for multiple LLM models
- **Multi-modal Support**: Image input support with validation
- **Real-time Testing**: Execute and test prompts with immediate feedback
- **Trace Integration**: View execution traces and debug information
- **Execute History**: Track execution history and results
- **File Upload**: Support for file attachments in prompts

### Prompt Playground

- **Experimentation Environment**: Test prompts without saving
- **Template Support**: Start from templates or scratch
- **Quick Iteration**: Rapid testing and refinement
- **Model Selection**: Choose from available models
- **Save to Library**: Convert playground prompts to saved prompts

### Observability Integration

- **Execution Traces**: View detailed trace logs for each prompt execution
- **Performance Metrics**: Track latency, tokens, and other metrics
- **Debug Information**: Access detailed execution information
- **Trace History Panel**: Browse historical executions
- **Direct Links**: Navigate to full trace details in observation system

## Routes

The application provides the following routes:

- `/` - Redirects to prompts list
- `/prompts` - Prompt list page (PE 列表)
- `/prompts/:promptID` - Prompt development page
- `/playground` - Prompt playground page

## Components

### Main Pages

- **PromptListPage**: Browse and manage all prompts with CRUD operations
- **PromptDevelopPage**: Full-featured prompt development environment
- **PromptPlaygroundPage**: Experimental prompt testing environment

### Custom Components

- **ExecuteHistoryPanel**: Display execution history with filtering
- **TraceTab**: Integrated trace viewing for prompt executions

### Integrated Components

From `@cozeloop/prompt-components-v2`:

- `PromptList`: Configurable prompt list with search and filters
- `PromptDevelop`: Complete prompt development UI
- `PromptCreateModal`: Modal for creating/editing prompts
- `PromptDeleteModal`: Confirmation modal for deletion

## API Reference

### Default Export

The package exports a single React component that provides the complete prompt engineering application with built-in routing.

```typescript
import PromptApp from '@cozeloop/prompt-pages';
```

### Key Features

- **Lazy Loading**: Pages are lazy-loaded for optimal performance
- **Workspace Integration**: Automatic workspace context integration
- **Model Management**: Dynamic model list loading per workspace
- **Event Tracking**: Built-in analytics event reporting
- **User Permissions**: Creator-based access control

## Dependencies

This package integrates with:

- `@cozeloop/prompt-components-v2`: Core prompt UI components and functionality
- `@cozeloop/observation-components`: Trace viewing and observability features
- `@cozeloop/observation-adapter`: Observability integration layer
- `@cozeloop/biz-components-adapter`: Business components (UserSelect, file upload)
- `@cozeloop/biz-hooks-adapter`: Business hooks (workspace, user, navigation, models)
- `@cozeloop/components`: Shared UI components
- `@cozeloop/hooks`: Utility hooks (modal, breadcrumb, refresh)
- `@cozeloop/i18n-adapter`: Internationalization support
- `@cozeloop/api-schema`: API type definitions
- `@cozeloop/toolkit`: Utility functions
- `@coze-arch/coze-design`: UI component library
- `react-router-dom`: Routing functionality

## Development

This package is built with:

- TypeScript for type safety
- React 18+ for UI components
- React Router for navigation with lazy loading
- Vitest for testing
- ESLint for code quality
- Coze Design System for UI components

### Scripts

```bash
# Build the package (currently a no-op)
npm run build

# Run linting
npm run lint

# Run tests (currently a no-op)
npm run test

# Run tests with coverage (currently a no-op)
npm run test:cov
```

## Contributing

This package is part of the CozeLoop monorepo. Please follow the monorepo contribution guidelines.

## License

Apache-2.0
