import { Button } from 'antd';

export const idRender = (
  dom: React.ReactNode,
  entity: Record<string, any>,
  setCurrentRow: any,
  setShowDetail: any,
) => {
  return (
    <Button
      key={entity.id}
      onClick={() => {
        setCurrentRow(entity);
        setShowDetail(true);
      }}
      type="link"
    >
      {dom}
    </Button>
  );
};
