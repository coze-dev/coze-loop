interface SelectorItemWrapperProps {
  title?: string;
  children: React.ReactNode;
  layoutMode?: 'horizontal' | 'vertical';
}

export const SelectorItemWrapper = ({
  title,
  children,
  layoutMode = 'horizontal',
}: SelectorItemWrapperProps) => {
  if (layoutMode === 'horizontal') {
    return children;
  }

  return (
    <div className="flex flex-col gap-y-2 w-full">
      <div className="text-[14px] coz-fg-primary font-medium leading-5">
        {title}
      </div>
      {children}
    </div>
  );
};
