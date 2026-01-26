// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { toBase64 } from 'js-base64';

const getFileName = (extension: string) =>
  `mermaid-diagram-${new Date().getTime()}.${extension}`;

const simulateDownload = (download: string, href: string): void => {
  const a = document.createElement('a');
  a.download = download;
  a.href = href;
  a.click();
  a.remove();
};

const downloadImage = (context, image) => () => {
  const { canvas } = context;
  context.drawImage(image, 0, 0, canvas.width, canvas.height);
  simulateDownload(
    getFileName('png'),
    canvas.toDataURL('image/png').replace('image/png', 'image/octet-stream'),
  );
};

const getBase64SVG = (
  svg: HTMLElement,
  width?: number,
  height?: number,
): string => {
  if (svg) {
    // Prevents the SVG size of the interface from being changed
    svg = svg.cloneNode(true) as HTMLElement;
  }
  height && svg?.setAttribute('height', `${height}px`);
  width && svg?.setAttribute('width', `${width}px`); // Workaround https://stackoverflow.com/questions/28690643/firefox-error-rendering-an-svg-image-to-html5-canvas-with-drawimage

  const svgString = svg.outerHTML
    .replaceAll('<br>', '<br/>')
    .replaceAll(/<img([^>]*)>/g, (m, g: string) => `<img ${g} />`);

  return toBase64(`<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet href="./font-awesome.all.min.css" type="text/css"?>
${svgString}`);
};

export const exportImage = async (svgSelector: string) => {
  await new Promise(resolve => setTimeout(resolve, 1000));
  const canvas = document.createElement('canvas');
  const svg = document.querySelector<HTMLElement>(svgSelector);
  if (!svg) {
    return;
  }

  const box = svg.getBoundingClientRect();

  const multiplier = 2;
  canvas.width = box.width * multiplier;
  canvas.height = box.height * multiplier;

  const context = canvas.getContext('2d');
  if (!context) {
    throw new Error('context not found');
  }

  context.fillStyle = 'white';
  context.fillRect(0, 0, canvas.width, canvas.height);

  const image = new Image();
  image.addEventListener('load', () => {
    downloadImage(context, image)();
  });
  image.src = `data:image/svg+xml;base64,${getBase64SVG(svg, canvas.width, canvas.height)}`;
  // Fallback to set panZoom to true after 2 seconds
  // This is a workaround for the case when the image is not loaded
};
