// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState, useRef } from 'react';

import styles from './custom-anchor.module.less';

interface AnchorLinkProps {
  href: string;
  title: string;
  isActive?: boolean;
  onClick?: () => void;
}

interface CustomAnchorProps {
  defaultAnchor?: string;
  getContainer?: () => HTMLElement | null;
  scrollMotion?: boolean;
  className?: string;
  onBeforeChange?: (currentLink: string) => void;
  onChange?: (currentLink: string) => void;
  children: React.ReactElement<AnchorLinkProps>[];
}

const SCROLL_TOP_OFFSET = 20;
const CLICK_SCROLL_DELAY = 500;

const CustomAnchor = ({
  defaultAnchor = '',
  getContainer,
  scrollMotion = false,
  className = '',
  onChange,
  children,
  onBeforeChange,
}: CustomAnchorProps) => {
  const [activeAnchor, setActiveAnchor] = useState(defaultAnchor);
  const isClickScrolling = useRef(false);

  const handleClick = (href: string) => {
    if (isClickScrolling.current) {
      return;
    }
    const anchor = href.replace('#', '');
    const element = document.getElementById(anchor);
    const container = getContainer?.() ?? window;

    if (element) {
      isClickScrolling.current = true;
      onBeforeChange?.(href);
      setActiveAnchor(href);
      onChange?.(href);

      setTimeout(() => {
        const scrollOptions: ScrollIntoViewOptions = {
          behavior: scrollMotion ? 'smooth' : 'auto',
          block: 'start',
        };

        if (container instanceof Window) {
          element.scrollIntoView(scrollOptions);
        } else {
          const { top: containerTop } = container.getBoundingClientRect();
          const { top: elementTop } = element.getBoundingClientRect();
          const { scrollTop } = container;
          const targetScrollTop =
            scrollTop + elementTop - containerTop - SCROLL_TOP_OFFSET;
          container.scrollTo({
            top: targetScrollTop,
            behavior: scrollMotion ? 'smooth' : 'auto',
          });
        }

        setTimeout(() => {
          isClickScrolling.current = false;
        }, CLICK_SCROLL_DELAY);
      }, 200);
    }
  };

  return (
    <div className={`${styles['custom-anchor']} ${className}`}>
      <div
        className="h-full w-[2px] absolute top-0 bottom-0 left-0"
        style={{
          background: 'var(--background-color-bg-4, #F6F8FA)',
        }}
      ></div>
      {React.Children.map(children, child =>
        React.cloneElement(child, {
          isActive: activeAnchor === child.props.href,
          onClick: () => handleClick(child.props.href),
        }),
      )}
    </div>
  );
};

const AnchorLink = ({ title, isActive = false, onClick }: AnchorLinkProps) => (
  <div
    className={`${styles['anchor-link']} ${isActive ? styles.active : ''}`}
    onClick={onClick}
  >
    {title}
  </div>
);

const CustomAnchorWithLink = Object.assign(CustomAnchor, { Link: AnchorLink });

export { CustomAnchorWithLink as CustomAnchor };
