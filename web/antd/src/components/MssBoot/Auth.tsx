import React, { useState } from 'react';
import { Tree } from 'antd';
import type { DataNode } from 'antd/es/tree';

export type AuthFormProps = {
  values: DataNode[];
  checkedKeys: React.Key[];
  setCheckedKeys: (checkedKeysValue: React.Key[]) => void;
};

const Auth: React.FC<AuthFormProps> = (props) => {
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  // const [checkedKeys, setCheckedKeys] = useState<React.Key[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([]);
  const [autoExpandParent, setAutoExpandParent] = useState<boolean>(true);

  const onExpand = (expandedKeysValue: React.Key[]) => {
    // console.log('onExpand', expandedKeysValue);
    // if not set autoExpandParent to false, if children expanded, parent can not collapse.
    // or, you can remove all expanded children keys.
    setExpandedKeys(expandedKeysValue);
    setAutoExpandParent(false);
  };

  const onCheck = (checkedKeysValue: React.Key[]) => {
    // console.log('onCheck', checkedKeysValue);
    props.setCheckedKeys(checkedKeysValue);
  };

  const onSelect = (selectedKeysValue: React.Key[]) => {
    // console.log('onSelect', info);
    setSelectedKeys(selectedKeysValue);
  };

  return (
    <Tree
      checkable
      onExpand={onExpand}
      expandedKeys={expandedKeys}
      autoExpandParent={autoExpandParent}
      // @ts-ignore
      onCheck={onCheck}
      checkedKeys={props.checkedKeys}
      onSelect={onSelect}
      selectedKeys={selectedKeys}
      treeData={props.values}
      defaultExpandAll={true}
    />
  );
};

export default Auth;
