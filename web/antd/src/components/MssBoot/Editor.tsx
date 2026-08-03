import BraftEditor, { BraftEditorProps } from 'braft-editor';
import 'braft-editor/dist/index.css';
import { getLocale, request } from '@umijs/max';
import React from 'react';

const RichTextEditor: React.FC<BraftEditorProps> = (props) => {
  const local = getLocale();
  const language = local.replace(/-.*$/, '');
  const media = {
    // @ts-ignore
    uploadFn: async (param) => {
      const formData = new FormData();
      formData.append('file', param.file);
      const response = await request('/admin/api/storage/upload', {
        method: 'POST',
        data: formData,
      });
      param.success({
        url: response,
        // @ts-ignore
        meta: null,
      });
    },
  };

  return (
    <BraftEditor
      {...props}
      value={BraftEditor.createEditorState(props.value)}
      language={language}
      media={media}
    />
  );
};

export default RichTextEditor;
