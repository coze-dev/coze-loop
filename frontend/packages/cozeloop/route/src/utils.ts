export function paddingPath(path: string) {
  if (path.startsWith('/')) {
    return path;
  }
  return `/${path}`;
}

export function getPath(path: string, baseURL: string) {
  if (!baseURL) {
    return path;
  }
  if (path.startsWith(baseURL)) {
    console.warn(`你可以直接使用${path.replace(`${baseURL}/`, '')}`);
    return path;
  }
  return `${baseURL}${paddingPath(path)}`;
}
