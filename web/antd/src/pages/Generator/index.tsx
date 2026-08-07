import React, { useState, useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  ProCard,
  ProFormSelect,
  ProFormText,
  StepsForm,
  ProForm,
} from '@ant-design/pro-components';
import { Button, message } from 'antd';
import {
  getTemplateGetBranches,
  getTemplateGetParams,
  getTemplateGetPath,
  postTemplateGenerate,
} from '@/services/admin/generator';
import { GithubOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { openOAuthAuthorization } from '@/utils/oauth';
import { runWithOneTimeCredential } from './credentialLifecycle';

const Generate: React.FC = () => {
  const intl = useIntl();
  const formRef = useRef<ProFormInstance>();

  const [branches, setBranches] = useState<string[]>([]);
  const [credential, setCredential] = useState<string>();
  const [credentialExpiresAt, setCredentialExpiresAt] = useState<string>();
  const [authorizing, setAuthorizing] = useState(false);
  const [paths, setPaths] = useState<string[]>([]);
  const [params, setParams] = useState<API.TemplateParam[]>([]);
  const [source, setSource] = useState<string>('');
  const [branch, setBranch] = useState<string>('');
  const [path, setPath] = useState<string>('');

  const getActiveCredential = () => {
    if (
      credential &&
      credentialExpiresAt &&
      Date.parse(credentialExpiresAt) <= Date.now()
    ) {
      setCredential(undefined);
      setCredentialExpiresAt(undefined);
      message.warning(intl.formatMessage({ id: 'pages.generator.githubAuth.expired' }));
      return undefined;
    }
    return credential;
  };

  const authorizeGithub = async () => {
    setAuthorizing(true);
    try {
      const result = await openOAuthAuthorization('github', 'integration');
      if (result.intent !== 'integration') {
        throw new Error('OAuth integration returned the wrong intent');
      }
      setCredential(result.credential);
      setCredentialExpiresAt(result.credentialExpiresAt);
      message.success(intl.formatMessage({ id: 'pages.generator.githubAuth.success' }));
    } catch {
      message.error(intl.formatMessage({ id: 'pages.generator.githubAuth.failed' }));
    } finally {
      setAuthorizing(false);
    }
  };

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
            const activeCredential = getActiveCredential();
            const req = await runWithOneTimeCredential({
              credential: activeCredential,
              request: (credentialHandle) =>
                postTemplateGenerate(
                  {
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
                  },
                  credentialHandle,
                  { skipErrorHandler: true },
                ),
              clearCredential: () => {
                setCredential(undefined);
                setCredentialExpiresAt(undefined);
              },
              onCredentialMissing: () => {
                message.warning(
                  intl.formatMessage({
                    id: 'pages.generator.githubAuth.required',
                  }),
                );
              },
              onCredentialFailure: () => {
                message.error(
                  intl.formatMessage({
                    id: 'pages.generator.githubAuth.consumedFailure',
                  }),
                );
              },
            });
            if (!req) {
              return false;
            }
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
              const sourceValue = formRef.current?.getFieldsValue().source;
              setSource(sourceValue);
              const branchesData = await getTemplateGetBranches(
                { source: sourceValue },
                getActiveCredential(),
              );
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
            <ProCard>
              <Button
                type="link"
                icon={<GithubOutlined />}
                loading={authorizing}
                onClick={() => void authorizeGithub()}
              >
                {intl.formatMessage({
                  id: credential
                    ? 'pages.generator.githubAuth.ready'
                    : 'pages.generator.githubAuth',
                })}
              </Button>
            </ProCard>
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
              const pathData = await getTemplateGetPath(
                {
                  branch: formRef.current?.getFieldsValue().branch,
                  source: source,
                },
                getActiveCredential(),
              );
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
              const paramsData = await getTemplateGetParams(
                {
                  path: formRef.current?.getFieldsValue().path,
                  source: source,
                  branch: branch,
                },
                getActiveCredential(),
              );
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
