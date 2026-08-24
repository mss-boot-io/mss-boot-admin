import { useIntl } from '@umijs/max';

const messages = {
  'presentation.title': ['Presentation', '页面展示'],
  'presentation.description': [
    'Govern presentation profiles—validate, draft, publish, and roll back—without code changes.',
    '无需改代码即可校验、保存、发布和回滚页面展示配置。',
  ],
  'presentation.forbidden': [
    'You cannot view page-presentation governance.',
    '你没有查看页面展示配置治理功能的权限。',
  ],
  'presentation.capabilities.empty': [
    'No pages expose presentation controls yet; runtime pages keep their compiled defaults.',
    '当前没有页面开放展示配置；运行时继续使用代码默认值。',
  ],
  'presentation.recovery.title': ['Presentation recovery is active', '展示恢复模式已启用'],
  'presentation.profile': ['Profile', '配置档案'],
  'presentation.state': ['State', '状态'],
  'presentation.version': ['Version', '版本'],
  'presentation.revision': ['Revision', '修订号'],
  'presentation.transition': ['Transition', '变更类型'],
  'presentation.actor': ['Actor', '操作人'],
  'presentation.createdAt': ['Created at', '创建时间'],
  'presentation.actions': ['Actions', '操作'],
  'presentation.valid': ['Valid', '有效'],
  'presentation.invalid': ['Invalid', '无效'],
  'presentation.profiles.title': ['Profiles', '配置档案'],
  'presentation.profiles.empty': ['No profiles yet', '尚未创建配置档案'],
  'presentation.create.action': ['New profile', '新建配置'],
  'presentation.create.title': ['Create draft', '新建展示草稿'],
  'presentation.editor.title': ['Presentation document', '展示配置文档'],
  'presentation.pageKey': ['Page capability', '页面能力'],
  'presentation.scope': ['Scope', '作用范围'],
  'presentation.subjectID': ['Role or user ID', '角色或用户 ID'],
  'presentation.scope.application': ['Application', '应用级'],
  'presentation.scope.role': ['Role', '角色级'],
  'presentation.scope.user': ['User', '用户级'],
  'presentation.validate.action': ['Validate', '校验'],
  'presentation.save.action': ['Save draft', '保存草稿'],
  'presentation.save.success': ['Draft saved', '展示草稿已保存'],
  'presentation.publish.action': ['Publish', '发布'],
  'presentation.publish.confirm': ['Publish this validated draft?', '发布这份已校验草稿？'],
  'presentation.publish.description': [
    'The new revision takes effect for this profile scope.',
    '新修订会在该配置档案范围内生效。',
  ],
  'presentation.publish.success': ['Revision published', '展示修订已发布'],
  'presentation.rollback.action': ['Roll back', '回滚'],
  'presentation.rollback.confirm': ['Republish revision {revision}?', '重新发布修订 {revision}？'],
  'presentation.rollback.description': [
    'Rollback adds a revision; existing history is unchanged.',
    '回滚会追加新修订，不改写已有历史。',
  ],
  'presentation.rollback.success': ['Revision republished', '历史修订已重新发布'],
  'presentation.unsaved': ['Unsaved local changes', '本地有未保存修改'],
  'presentation.document': ['Presentation profile JSON', '页面展示配置 JSON'],
  'presentation.validation.valid': ['Document is valid', '文档校验通过'],
  'presentation.validation.invalid': ['Document is invalid', '文档校验失败'],
  'presentation.preview.title': ['Trusted preview', '可信预览'],
  'presentation.preview.pageTitle': ['Page title', '页面标题'],
  'presentation.preview.dataSource': ['Data source', '数据源'],
  'presentation.preview.density': ['List density', '列表密度'],
  'presentation.preview.pageSize': ['Page size', '每页条数'],
  'presentation.preview.columns': ['Visible columns', '可见列'],
  'presentation.preview.actions': ['Available actions', '可用操作'],
  'presentation.conflict.title': ['Server has a newer version', '服务端已有新版本'],
  'presentation.conflict.description': [
    'Local JSON is preserved. Server version: {version}.',
    '本地 JSON 已保留；服务端版本为 {version}。',
  ],
  'presentation.conflict.reload': ['Discard and load latest', '丢弃并加载最新版本'],
  'presentation.conflict.discard.confirm': [
    'Discard local JSON and load the latest version?',
    '丢弃本地 JSON 并加载服务端最新版本？',
  ],
  'presentation.conflict.discard.description': [
    'This action replaces the local editor contents.',
    '此操作会替换编辑器中的本地内容。',
  ],
  'presentation.operation.failed': ['Operation failed', '操作失败'],
  'presentation.history.title': ['Publication history', '发布历史'],
  'presentation.history.empty': ['No revisions yet', '尚无修订'],
} as const;

type PresentationMessageID = keyof typeof messages;

function formatBundledMessage(
  template: string,
  values?: Parameters<ReturnType<typeof useIntl>['formatMessage']>[1],
): string {
  return template.replace(/\{([A-Za-z][A-Za-z0-9]*)\}/g, (placeholder, key: string) => {
    const value = values?.[key];
    return value === undefined || value === null ? placeholder : String(value);
  });
}

export function usePresentationIntl() {
  const intl = useIntl();
  const locale = typeof intl.locale === 'string' ? intl.locale : 'en-US';
  const languageIndex = locale.startsWith('zh') ? 1 : 0;
  const formatMessage = ((
    descriptor: Parameters<typeof intl.formatMessage>[0],
    values?: Parameters<typeof intl.formatMessage>[1],
  ) => {
    const { id } = descriptor;
    if (typeof id === 'string' && Object.hasOwn(messages, id)) {
      const defaultMessage = messages[id as PresentationMessageID][languageIndex];
      if (Object.hasOwn(intl, 'messages')) {
        return formatBundledMessage(defaultMessage, values);
      }
      return intl.formatMessage({ ...descriptor, defaultMessage }, values);
    }
    return intl.formatMessage(descriptor, values);
  }) as typeof intl.formatMessage;

  return { formatMessage, locale };
}
