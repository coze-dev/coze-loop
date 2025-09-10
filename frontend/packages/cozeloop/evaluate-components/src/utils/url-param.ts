export const getUrlParamWithDelete = (key: string) => {
  const queryString = window.location.search;
  const urlParams = new URLSearchParams(queryString);
  const value = urlParams.get(key);
  urlParams.delete(key);
  window.history.pushState({}, '', urlParams.toString());
  return value;
};

export const getUrlParam = (key: string) => {
  const searchParams = new URLSearchParams(window.location.search);
  return searchParams.get(key);
};
