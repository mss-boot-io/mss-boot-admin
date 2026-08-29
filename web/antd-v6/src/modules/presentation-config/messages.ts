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
  'presentation.editor.mode.visual': ['Visual editor', '可视化编辑'],
  'presentation.editor.mode.raw': ['Raw JSON', '原始 JSON'],
  'presentation.editor.visual.unavailable': [
    'The document is not safe to open visually. Fix it in Raw JSON first.',
    '当前文档无法安全进入可视化编辑，请先在原始 JSON 中修复。',
  ],
  'presentation.validation.openRaw': ['Open raw path', '定位原始路径'],
  'presentation.visual.general': ['General', '常规'],
  'presentation.visual.list': ['List', '列表'],
  'presentation.visual.search': ['Search', '搜索'],
  'presentation.visual.form': ['Form', '表单'],
  'presentation.visual.detail': ['Detail', '详情'],
  'presentation.visual.actions': ['Actions', '动作'],
  'presentation.visual.title': ['Page title', '页面标题'],
  'presentation.visual.dataSource': ['Data source', '数据源'],
  'presentation.visual.list.settings': ['List settings', '列表设置'],
  'presentation.visual.search.settings': ['Search settings', '搜索设置'],
  'presentation.visual.layout': ['Layout', '布局'],
  'presentation.visual.density': ['Density', '密度'],
  'presentation.visual.pageSize': ['Page size', '每页条数'],
  'presentation.visual.defaultSort': ['Default sort', '默认排序'],
  'presentation.visual.sort.field': ['Sort field', '排序字段'],
  'presentation.visual.sort.direction': ['Sort direction', '排序方向'],
  'presentation.visual.sort.add': ['Add sort', '添加排序'],
  'presentation.visual.collapsed': ['Collapsed by default', '默认收起'],
  'presentation.visual.columns': ['Layout columns', '布局列数'],
  'presentation.visual.list.fields': ['List columns', '列表列'],
  'presentation.visual.search.fields': ['Search fields', '搜索字段'],
  'presentation.visual.form.fields': ['Form fields', '表单字段'],
  'presentation.visual.detail.fields': ['Detail fields', '详情字段'],
  'presentation.visual.label': ['Label', '标签'],
  'presentation.visual.component': ['Component', '组件'],
  'presentation.visual.order': ['Order', '顺序'],
  'presentation.visual.hidden': ['Hidden', '隐藏'],
  'presentation.visual.width': ['Width', '宽度'],
  'presentation.visual.span': ['Grid span', '栅格跨度'],
  'presentation.visual.placeholder': ['Placeholder', '占位提示'],
  'presentation.visual.help': ['Help text', '帮助文字'],
  'presentation.visual.placement': ['Placement', '位置'],
  'presentation.visual.confirm': ['Confirmation text', '确认文字'],
  'presentation.visual.visibleWhen': ['Visibility condition', '可见条件'],
  'presentation.visual.condition.add': ['Add condition', '添加条件'],
  'presentation.visual.condition.field': ['Condition field', '条件字段'],
  'presentation.visual.condition.operator': ['Condition operator', '条件运算符'],
  'presentation.visual.condition.value': ['Condition JSON value', '条件 JSON 值'],
  'presentation.visual.condition.value.invalid': [
    'Enter a bounded JSON scalar, or a scalar array for membership.',
    '请输入有界 JSON 标量；成员运算请输入标量数组。',
  ],
  'presentation.visual.condition.compound': [
    'Compound condition preserved; use Raw JSON to edit it, or replace it with one predicate.',
    '复合条件已原样保留；请在原始 JSON 中编辑，或替换为单个谓词。',
  ],
  'presentation.visual.condition.replace': ['Replace with predicate', '替换为单个谓词'],
  'presentation.visual.condition.list.disabled': [
    'Version 2 does not allow conditions on list columns.',
    '版本 2 不允许列表列使用条件。',
  ],
  'presentation.visual.condition.required.disabled': [
    'Required form fields cannot be hidden or conditional.',
    '必填表单字段不能隐藏或设置可见条件。',
  ],
  'presentation.visual.condition.toolbar.disabled': [
    'Version 2 does not allow conditions on toolbar actions.',
    '版本 2 不允许工具栏动作使用条件。',
  ],
  'presentation.visual.condition.toolbar.invalid': [
    'A preserved condition conflicts with toolbar placement; locate it in Raw JSON.',
    '已保留的条件与工具栏位置冲突，请在原始 JSON 中定位处理。',
  ],
  'presentation.visual.inherit': ['Inherit', '继承'],
  'presentation.visual.inherited': ['Inherited', '继承值'],
  'presentation.visual.overridden': ['Overridden', '已覆盖'],
  'presentation.visual.override': ['Override', '覆盖'],
  'presentation.visual.true': ['Yes', '是'],
  'presentation.visual.false': ['No', '否'],
  'presentation.visual.remove': ['Remove', '移除'],
  'presentation.visual.restore.field': ['Restore field', '恢复字段继承'],
  'presentation.visual.restore.action': ['Restore action', '恢复动作继承'],
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
