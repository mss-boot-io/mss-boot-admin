import {
  type PageContainerProps,
  PageContainer as ProPageContainer,
} from '@ant-design/pro-components';

function semanticTitle(title: PageContainerProps['title']) {
  if (title === undefined || title === null || title === false) return title;
  return (
    <h1
      style={{
        color: 'inherit',
        font: 'inherit',
        margin: 0,
      }}
    >
      {title}
    </h1>
  );
}

/**
 * Keeps ProComponents layout behavior while making the visible page title the
 * document's primary heading. Application pages should use this adapter rather
 * than depending on ProComponents' presentational title span.
 */
export function PageContainer({ title, ...props }: PageContainerProps) {
  return <ProPageContainer {...props} title={semanticTitle(title)} />;
}

export type { PageContainerProps };
