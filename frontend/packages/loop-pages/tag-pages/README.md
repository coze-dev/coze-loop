# @cozeloop/tag-pages

Tag management pages for CozeLoop platform, providing tag organization, categorization, and CRUD operations.

## Overview

This package provides the tag management system pages for the CozeLoop platform. It offers a complete workflow for creating, viewing, editing, and organizing tags to help categorize and manage resources within the workspace. The package includes a tag list view, detail pages, and creation interfaces.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/tag-pages": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import TagApp from '@cozeloop/tag-pages';
import { BrowserRouter } from 'react-router-dom';

function App() {
  return (
    <BrowserRouter>
      <TagApp />
    </BrowserRouter>
  );
}
```

The app provides comprehensive tag management capabilities with automatic routing configuration.

## Features

### Tag Management

- **Tag List View**: Browse and manage all tags in your workspace
- **Tag Creation**: Create new tags with customizable properties
- **Tag Details**: View and edit individual tag information
- **Tag Organization**: Organize and categorize resources using tags
- **Consistent Navigation**: Integrated breadcrumb and navigation system

## Routes

The application provides the following routes:

- `/` - Redirects to tag list
- `/tag` - Tag list page (标签管理)
- `/tag/create` - Create new tag page
- `/tag/:tagId` - Tag detail and edit page

**Note**: The actual complete route path is `tag/tag/*` due to the module base path prefix.

## Components

### Integrated Pages

From `@cozeloop/tag-components`:

- **TagsListPage**: Browse and manage all tags
- **TagsCreatePage**: Create new tags
- **TagsDetailPage**: View and edit tag details

**Note**: Per the codebase TODO, page-level components are currently exported from `@cozeloop/tag-components` but should be moved to this package in future refactoring.

## API Reference

### Default Export

The package exports a single React component that provides the complete tag management application with built-in routing.

```typescript
import TagApp from '@cozeloop/tag-pages';
```

### Route Configuration

The tag module uses a consistent route configuration:

- **Module Base Path**: `tag`
- **Tag List Path**: `tag/tag` (used for navigation and breadcrumbs)
- All pages receive `tagListPagePath` prop for consistent navigation

## Dependencies

This package integrates with:

- `@cozeloop/tag-components`: Core tag UI components and page implementations
- `react-router-dom`: Routing functionality

### Development Dependencies

- `@cozeloop/i18n-adapter`: Internationalization support
- `@coze-arch/eslint-config`: ESLint configuration
- `@coze-arch/ts-config`: TypeScript configuration
- `@coze-arch/vitest-config`: Testing configuration

## Development

This package is built with:

- TypeScript for type safety
- React 18+ for UI components
- React Router for navigation
- Vitest for testing
- ESLint for code quality
- Storybook for component development

### Scripts

```bash
# Build the package
npm run build

# Run Storybook development server
npm run dev

# Run linting
npm run lint

# Run tests
npm run test

# Run tests with coverage
npm run test:cov
```

## Architecture Notes

This package currently serves as a routing wrapper around `@cozeloop/tag-components`. According to the codebase TODO, future refactoring should:

- Move page-level components from `@cozeloop/tag-components` to this package
- Keep only UI components in `@cozeloop/tag-components`
- Improve separation of concerns between components and pages

## Contributing

This package is part of the CozeLoop monorepo. Please follow the monorepo contribution guidelines.

## License

Apache-2.0
