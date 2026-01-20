# @cozeloop/evaluate-pages

Evaluation and testing pages for CozeLoop platform, providing comprehensive experiment management, dataset handling, and evaluator configuration.

## Overview

This package provides the complete evaluation system pages for the CozeLoop platform. It integrates evaluation workflows including dataset management, evaluator creation and configuration, and experiment execution and analysis. The package serves as the main entry point for the evaluation feature set in CozeLoop.

## Getting Started

### Installation

Add this package to your `package.json`:

```json
{
  "dependencies": {
    "@cozeloop/evaluate-pages": "workspace:*"
  }
}
```

Then run:

```bash
rush update
```

### Usage

```typescript
import EvaluateApp from '@cozeloop/evaluate-pages';
import { BrowserRouter } from 'react-router-dom';

function App() {
  return (
    <BrowserRouter>
      <EvaluateApp />
    </BrowserRouter>
  );
}
```

The app provides comprehensive routing for the evaluation workflow with automatic initialization of community evaluation configurations.

## Features

### Dataset Management (评测集)

- **Dataset List**: Browse and manage all evaluation datasets
- **Dataset Creation**: Create new datasets for evaluation
- **Dataset Details**: View and edit dataset configurations and data

### Evaluator Management (评估器)

- **Evaluator List**: View all available evaluators
- **LLM Evaluator Creation**: Create and configure LLM-based evaluators
- **Code Evaluator Creation**: Create custom code-based evaluators
- **Evaluator Details**: View and modify evaluator configurations

### Experiment Management (实验)

- **Experiment List**: Browse all experiments
- **Experiment Creation**: Set up new evaluation experiments
- **Experiment Details**: View experiment results and analytics
- **Experiment Contrast**: Compare multiple experiments side-by-side

## Routes

The application provides the following routes:

### Datasets (评测集)

- `/datasets` - Dataset list page
- `/datasets/create` - Create new dataset
- `/datasets/:id` - Dataset detail page

### Evaluators (评估器)

- `/evaluators` - Evaluator list page
- `/evaluators/create/llm` - Create LLM evaluator
- `/evaluators/create/llm/:id?` - Edit LLM evaluator
- `/evaluators/create/code` - Create code evaluator
- `/evaluators/create/code/:id?` - Edit code evaluator
- `/evaluators/:id` - LLM evaluator detail page
- `/evaluators/code/:id?` - Code evaluator detail page

### Experiments (实验)

- `/experiments` - Redirects to experiments list
- `/experiments/list` - Experiment list page
- `/experiments/create` - Create new experiment
- `/experiments/:experimentID` - Experiment detail page
- `/experiments/contrast` - Compare experiments

## API Reference

### Default Export

The package exports a single React component that provides the complete evaluation application with built-in routing and community configuration initialization.

```typescript
import EvaluateApp from '@cozeloop/evaluate-pages';
```

### Configuration

The app automatically initializes community evaluation configurations using the `useEvaluateConfigCommunityInit` hook from `@cozeloop/evaluate`.

## Dependencies

This package integrates with:

- `@cozeloop/evaluate`: Core evaluation functionality and page components
- `@cozeloop/evaluate-components`: UI components for evaluation features
- `@cozeloop/i18n-adapter`: Internationalization support
- `@cozeloop/toolkit`: Utility functions
- `@cozeloop/biz-hooks-adapter`: Business logic hooks
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
