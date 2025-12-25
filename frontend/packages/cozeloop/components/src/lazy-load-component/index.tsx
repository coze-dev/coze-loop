// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState, useEffect, useRef } from 'react';

import { Skeleton } from '@coze-arch/coze-design';

export const LazyLoadComponent = ({
  children,
  placeholder,
}: {
  children: React.ReactNode;
  placeholder?: React.ReactNode;
}) => {
  const [isVisible, setIsVisible] = useState(false);
  const domRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if ('IntersectionObserver' in window) {
      if (!domRef.current) {
        return;
      }
      // 使用 Intersection Observer
      const observer = new IntersectionObserver(entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting) {
            setIsVisible(true);
            observer.unobserve(domRef.current as Element);
          }
        });
      });

      observer.observe(domRef.current);
      return () => observer.disconnect();
    } else {
      // 回退到 scroll 事件监听
      const handleScroll = () => {
        if (domRef.current) {
          const rect = domRef.current.getBoundingClientRect();
          if (rect.top <= window.innerHeight && rect.bottom >= 0) {
            setIsVisible(true);
            window.removeEventListener('scroll', handleScroll);
          }
        }
      };

      handleScroll(); // 初始化检查
      document.addEventListener('scroll', handleScroll);
      return () => document.removeEventListener('scroll', handleScroll);
    }
  }, []);

  return (
    <div ref={domRef}>
      {isVisible ? children : <Skeleton placeholder={placeholder} />}
    </div>
  );
};
