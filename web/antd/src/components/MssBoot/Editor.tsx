import { message } from 'antd';
import React, { useCallback, useMemo, useRef, useState } from 'react';
import ReactQuill from 'react-quill-new';
import 'react-quill-new/dist/quill.snow.css';
import { request } from '@umijs/max';

type QuillProps = React.ComponentProps<typeof ReactQuill>;

type LegacyEditorValue =
  | string
  | {
      toHTML: () => string;
    };

export type RichTextEditorProps = Omit<
  QuillProps,
  'defaultValue' | 'modules' | 'onChange' | 'value'
> & {
  defaultValue?: LegacyEditorValue;
  modules?: QuillProps['modules'];
  onChange?: (html: string) => void;
  value?: LegacyEditorValue;
};

type QuillSelection = {
  index: number;
  length: number;
};

type QuillEditor = {
  getLength: () => number;
  getSelection: (focus?: boolean) => QuillSelection | null;
  insertEmbed: (index: number, type: string, value: string, source?: string) => void;
  setSelection: (index: number, length?: number, source?: string) => void;
};

type QuillComponent = {
  getEditor: () => QuillEditor;
};

type StorageUploadResponse =
  | string
  | {
      data?: {
        url?: string;
      };
      url?: string;
    };

const toolbar = [
  [{ header: [1, 2, 3, false] }],
  ['bold', 'italic', 'underline', 'strike'],
  [{ list: 'ordered' }, { list: 'bullet' }],
  [{ color: [] }, { background: [] }],
  ['blockquote', 'code-block'],
  ['link', 'image'],
  ['clean'],
];

const normalizeValue = (value?: LegacyEditorValue): string => {
  if (typeof value === 'string') {
    return value;
  }
  if (value && typeof value.toHTML === 'function') {
    return value.toHTML();
  }
  return '';
};

const resolveUploadURL = (response: StorageUploadResponse): string => {
  if (typeof response === 'string') {
    return response;
  }
  return response.url || response.data?.url || '';
};

const RichTextEditor: React.FC<RichTextEditorProps> = ({
  defaultValue,
  modules,
  onChange,
  theme = 'snow',
  value,
  ...quillProps
}) => {
  const editorRef = useRef<QuillComponent | null>(null);
  const controlled = value !== undefined;
  const [internalValue, setInternalValue] = useState(() => normalizeValue(defaultValue));

  const uploadImage = useCallback(() => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.onchange = async () => {
      const file = input.files?.[0];
      if (!file) {
        return;
      }

      try {
        const formData = new FormData();
        formData.append('file', file);
        const response = await request<StorageUploadResponse>('/admin/api/storage/upload', {
          method: 'POST',
          data: formData,
        });
        const url = resolveUploadURL(response);
        if (!url) {
          throw new Error('Upload response did not contain a URL');
        }

        const editor = editorRef.current?.getEditor();
        if (!editor) {
          return;
        }
        const selection = editor.getSelection(true);
        const index = selection?.index ?? Math.max(editor.getLength() - 1, 0);
        editor.insertEmbed(index, 'image', url, 'user');
        editor.setSelection(index + 1, 0, 'silent');
      } catch (error) {
        const detail = error instanceof Error ? error.message : String(error);
        message.error(`Image upload failed: ${detail}`);
      }
    };
    input.click();
  }, []);

  const editorModules = useMemo<QuillProps['modules']>(() => {
    if (modules) {
      return modules;
    }
    return {
      toolbar: {
        container: toolbar,
        handlers: {
          image: uploadImage,
        },
      },
    };
  }, [modules, uploadImage]);

  const handleChange = useCallback(
    (html: string) => {
      if (!controlled) {
        setInternalValue(html);
      }
      onChange?.(html);
    },
    [controlled, onChange],
  );

  return (
    <ReactQuill
      {...quillProps}
      ref={(instance) => {
        editorRef.current = instance as unknown as QuillComponent | null;
      }}
      modules={editorModules}
      onChange={handleChange}
      theme={theme}
      value={controlled ? normalizeValue(value) : internalValue}
    />
  );
};

export default RichTextEditor;
