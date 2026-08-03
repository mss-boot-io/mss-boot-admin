import React, { useState, useRef, useEffect } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  ProCard,
  ProFormSelect,
  ProFormText,
  StepsForm,
  ProForm,
} from '@ant-design/pro-components';
import { message } from 'antd';
import {
  getGithubGetLoginUrl,
  getTemplateGetBranches,
  getTemplateGetParams,
  getTemplateGetPath,
  postTemplateGenerate,
} from '@/services/admin/generator';
import { GithubOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';

function randToken(): string {
  const buffer = new Uint8Array(32);
  window.crypto.getRandomValues(buffer);
  // @ts-ignore
  return btoa(String.fromCharCode.apply(null, buffer));
}

const Generate: React.FC = () => {
  const intl = useIntl();
  const formRef = useRef<ProFormInstance>();

  const [branches, setBranches] = useState<string[]>([]);
  const [accessToken, setAccessToken] = useState<string>('');
  const [paths, setPaths] = useState<string[]>([]);
  const [params, setParams] = useState<API.TemplateParam[]>([]);
  const [source, setSource] = useState<string>('');
  const [branch, setBranch] = useState<string>('');
  const [path, setPath] = useState<string>('');

  useEffect(() => {
    if (!accessToken) {
      const intervalId = setInterval(() => {
        const token = localStorage.getItem('github.token');
        if (token) {
          setAccessToken(token);
          formRef.current?.setFieldsValue({ accessToken: token });
          clearInterval(intervalId);
        }
      }, 1000);

      return () => {
        clearInterval(intervalId);
      };
    }
  }, [accessToken, setAccessToken]);

  return (
    <div>
      <ProCard>
        <StepsForm<{
          name: string;
        }>
          formRef={formRef}
          onFinish={async () => {
            const ps = formRef.current?.getFieldsValue();
            delete ps.repo;
            delete ps.service;
            const req = await postTemplateGenerate({
              accessToken: accessToken,
              email: formRef.current?.getFieldsValue().email,
              generate: {
                params: ps,
                repo: formRef.current?.getFieldsValue().repo,
                service: formRef.current?.getFieldsValue().service,
              },
              template: {
                source: source,
                branch: branch,
                path: path,
              },
            });
            if (req.repo && req.branch) {
              message.success(
                intl.formatMessage({ id: 'pages.generator.success' }, { branch: req.branch }),
              );
            }
          }}
          formProps={{
            validateMessages: {
              required: '此项为必填项',
            },
          }}
        >
          <StepsForm.StepForm<{
            name: string;
          }>
            name="template"
            title={intl.formatMessage({ id: 'pages.generator.steps.template.title' })}
            stepProps={{
              description: intl.formatMessage({ id: 'pages.generator.steps.template.desc' }),
            }}
            onFinish={async () => {
              if (!accessToken) {
                const token = localStorage.getItem('github.token');
                if (token) {
                  setAccessToken(token);
                }
              }
              formRef.current?.setFieldsValue({ accessToken });
              setSource(formRef.current?.getFieldsValue().source);
              const data = formRef.current?.getFieldsValue();
              data.accessToken = accessToken;
              // console.log(data);
              const branchesData = await getTemplateGetBranches(data);
              // console.log(branchesData);
              setBranches(branchesData.branches || []);
              return true;
            }}
          >
            <ProFormText
              name="source"
              label={intl.formatMessage({ id: 'pages.generator.steps.template.title' })}
              width="md"
              tooltip="目前支持github地址"
              placeholder={intl.formatMessage({ id: 'pages.form.placeholder' })}
              rules={[{ required: true }]}
            />
            {accessToken ? (
              ''
            ) : (
              <ProCard
                onClick={async () => {
                  // console.log(location);
                  const state = randToken();
                  localStorage.setItem('github.state', state);
                  const loginURL = await getGithubGetLoginUrl({ state: state });
                  const w = window.open('about:blank');
                  // @ts-ignore
                  w.location.href = loginURL;
                }}
              >
                {intl.formatMessage({ id: 'pages.generator.githubAuth' })}{' '}
                <GithubOutlined key="GithubOutlined" />
              </ProCard>
            )}
          </StepsForm.StepForm>
          <StepsForm.StepForm<{
            checkbox: string;
          }>
            name="branch"
            title={intl.formatMessage({ id: 'pages.generator.steps.branch.title' })}
            stepProps={{
              description: intl.formatMessage({ id: 'pages.generator.steps.branch.desc' }),
            }}
            onFinish={async () => {
              setBranch(formRef.current?.getFieldsValue().branch);
              const pathData = await getTemplateGetPath({
                branch: formRef.current?.getFieldsValue().branch,
                source: source,
                accessToken: accessToken,
              });
              setPaths(pathData.path || []);

              // console.log(formRef.current?.getFieldsValue());
              return true;
            }}
          >
            <ProFormSelect
              label={intl.formatMessage({ id: 'pages.generator.steps.branch.title' })}
              name="branch"
              rules={[
                {
                  required: true,
                },
              ]}
              initialValue="请选择"
              options={branches}
            />
          </StepsForm.StepForm>
          <StepsForm.StepForm
            name="path"
            title={intl.formatMessage({ id: 'pages.generator.steps.path.title' })}
            stepProps={{
              description: intl.formatMessage({ id: 'pages.generator.steps.path.desc' }),
            }}
            onFinish={async () => {
              setPath(formRef.current?.getFieldsValue().path);
              const paramsData = await getTemplateGetParams({
                path: formRef.current?.getFieldsValue().path,
                source: source,
                branch: branch,
                accessToken: accessToken,
              });
              setParams(paramsData.params || []);

              // console.log(formRef.current?.getFieldsValue());
              return true;
            }}
          >
            <ProFormSelect
              label={intl.formatMessage({ id: 'pages.generator.steps.path.title' })}
              name="path"
              rules={[
                {
                  required: true,
                },
              ]}
              initialValue="请选择"
              options={paths}
            />
          </StepsForm.StepForm>
          <StepsForm.StepForm
            name="params"
            title={intl.formatMessage({ id: 'pages.generator.steps.params.title' })}
            stepProps={{
              description: intl.formatMessage({ id: 'pages.generator.steps.params.desc' }),
            }}
          >
            <ProCard
              title={intl.formatMessage({ id: 'pages.generator.steps.params.title' })}
              tooltip={intl.formatMessage({ id: 'pages.generator.steps.params.tooltip' })}
              style={{ maxWidth: 500 }}
            >
              <ProForm.Group>
                <ProFormText
                  name="repo"
                  label={intl.formatMessage({ id: 'pages.generator.repo' })}
                  tooltip={intl.formatMessage({ id: 'pages.generator.repo.tooltip' })}
                />
              </ProForm.Group>
              <ProForm.Group>
                <ProFormText
                  name="service"
                  label={intl.formatMessage({ id: 'pages.generator.service' })}
                  tooltip={intl.formatMessage({ id: 'pages.generator.service.tooltip' })}
                />
              </ProForm.Group>
              <ProForm.Group>
                <ProFormText
                  name="email"
                  label={intl.formatMessage({ id: 'pages.generator.email' })}
                  tooltip={intl.formatMessage({ id: 'pages.generator.email.tooltip' })}
                />
              </ProForm.Group>
            </ProCard>

            <ProCard
              title={intl.formatMessage({ id: 'pages.generator.steps.params.title' })}
              tooltip={intl.formatMessage({ id: 'pages.generator.steps.params.tooltip' })}
              style={{ maxWidth: 500 }}
            >
              {params.map((item) => (
                <ProForm.Group key={item.name}>
                  <ProFormText
                    key={item.name}
                    name={item.name}
                    label={item.name}
                    tooltip={item.tip}
                  />
                </ProForm.Group>
              ))}
            </ProCard>
          </StepsForm.StepForm>
        </StepsForm>
      </ProCard>
    </div>
  );
};

export default Generate;
