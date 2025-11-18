# 🧭 Cozeloop Frontend

English | [简体中文](./README.zh-CN.md)

This is a Monorepo managed by [Rush.js](https://rushjs.io/).

## 🚀 Getting Started

```
_____________________________________
< Getting Started >
-------------------------------------
       \   ^__^
        \  (oo)\_______
           (__)\       )\/\
               ||----w |
               ||     ||
```

Environment requirements:

* Node.js 18+ (lts/iron recommended)
* pnpm 8.15.8
* Rush 5.147.1

### 1. Install Node.js 18+

``` bash
nvm install lts/iron
nvm alias default lts/iron # set default Node version
nvm use lts/iron
```

### 2. Checkout repository and change to frontend dir

```bash
# clone the repository
git clone git@github.com:coze-dev/coze-loop.git

# change current work dir
cd frontend
```

### 3. Install required global dependencies

```bash
npm i -g pnpm@8.15.8 @microsoft/rush@5.147.1
```

### 4. Install/Update project dependencies

```bash
rush update
```

## 🔨 Development

### 1. Run

> Tip: using `rushx` instead of `pnpm run` or `npm run`

The cozeloop project in `apps/cozeloop` is a React App, start development by running:

```bash
cd apps/cozeloop

rushx dev
```

Open [http://localhost:8090/](http://localhost:8090/) with your browser to see the page.

### 2. Build

The cozeloop project is built with [Rsbuild](https://rsbuild.dev/), and the config file is [apps/cozeloop/rsbuild.config.ts](./apps/cozeloop/rsbuild.config.ts).

```bash
cd apps/cozeloop

rushx build
```

### 3. Dependencies in workspace

As you can see, many dependencies listed in [apps/cozeloop/package.json](./apps/cozeloop/package.json) are versioned by `workspace:*`, which means they are maintained inner this repository.

Change the source code of workspace dependencies and the changes will work directly (sometimes re-run is required), since the cozeloop project depends the source codes rather than artifacts.

### 4. Update api by idl

Run the following command, and the idl files will be generated in [packages/cozeloop/api-schema](./packages/cozeloop/api-schema).

```bash
rush update-api
```

## 📄 License

* [Apache License, Version 2.0](../LICENSE)
