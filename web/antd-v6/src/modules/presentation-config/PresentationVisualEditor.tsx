import {
  isLimitedTablePresentationCapability,
  type LocalizedText,
  type PageActionPresentation,
  type PageCapabilityAction,
  type PageCapabilityDefinition,
  type PageCapabilityField,
  type PageFieldPresentation,
  type PageSort,
  type PresentationActionPlacement,
  type PresentationCondition,
  type PresentationDensity,
  type PresentationSurface,
} from '@mss-admin-core/shared/presentation/contract';
import {
  Alert,
  Button,
  Card,
  Col,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import type { PresentationCapability, PresentationJSONValue } from './contract';
import { usePresentationIntl } from './messages';
import PresentationConditionEditor from './PresentationConditionEditor';
import {
  type PresentationDraftAST,
  presentationActionOverride,
  presentationDraftSection,
  presentationDraftSpec,
  presentationFieldOverride,
  resetPresentationActionOverrides,
  resetPresentationFieldOverrides,
  setPresentationActionCondition,
  setPresentationActionLocalizedOverride,
  setPresentationActionOverride,
  setPresentationFieldCondition,
  setPresentationFieldLocalizedOverride,
  setPresentationFieldOverride,
  setPresentationLocalizedOverride,
  setPresentationSpecOverride,
} from './presentationDraftAst';

interface PresentationVisualEditorProps {
  capability: PresentationCapability;
  document: PresentationDraftAST;
  onChange: (document: PresentationDraftAST) => void;
}

type FieldSurface = PresentationSurface;
type FormatMessage = ReturnType<typeof usePresentationIntl>['formatMessage'];

const surfaces: readonly FieldSurface[] = ['list', 'search', 'form', 'detail'];

function isObject(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOwn(value: Readonly<Record<string, unknown>> | undefined, key: string): boolean {
  return Boolean(value && Object.hasOwn(value, key));
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function numberValue(value: unknown): number | undefined {
  return typeof value === 'number' ? value : undefined;
}

function booleanValue(value: unknown): boolean | undefined {
  return typeof value === 'boolean' ? value : undefined;
}

function localizedValue(value: unknown, locale: keyof LocalizedText): string | undefined {
  if (!isObject(value)) return undefined;
  return typeof value[locale] === 'string' ? value[locale] : undefined;
}

function conditionValue(value: unknown): PresentationCondition | undefined {
  return isObject(value) ? (value as unknown as PresentationCondition) : undefined;
}

function defaultFields(
  definition: PageCapabilityDefinition,
  surface: FieldSurface,
): readonly PageFieldPresentation[] {
  if (surface === 'list') return definition.defaultPresentation.list.columns;
  return definition.defaultPresentation[surface].fields;
}

function fieldCollectionKey(surface: FieldSurface): 'columns' | 'fields' {
  return surface === 'list' ? 'columns' : 'fields';
}

function rawFieldPath(
  document: PresentationDraftAST,
  surface: FieldSurface,
  fieldID: string,
): string {
  const collection = presentationDraftSection(document, surface)[fieldCollectionKey(surface)];
  const index = Array.isArray(collection)
    ? collection.findIndex((entry) => isObject(entry) && entry.field === fieldID)
    : -1;
  return `spec.${surface}.${fieldCollectionKey(surface)}${index >= 0 ? `[${index}]` : ''}.visibleWhen`;
}

function rawActionPath(document: PresentationDraftAST, actionID: string): string {
  const actions = presentationDraftSpec(document).actions;
  const index = Array.isArray(actions)
    ? actions.findIndex((entry) => isObject(entry) && entry.action === actionID)
    : -1;
  return `spec.actions${index >= 0 ? `[${index}]` : ''}.visibleWhen`;
}

function inheritedLabel(formatMessage: FormatMessage, inherited: boolean, fallback: unknown) {
  return inherited ? (
    <Tag>
      {formatMessage({ id: 'presentation.visual.inherited' })}: {String(fallback ?? '—')}
    </Tag>
  ) : (
    <Tag color="blue">{formatMessage({ id: 'presentation.visual.overridden' })}</Tag>
  );
}

function LocalizedInputs({
  ariaPrefix,
  value,
  onChange,
}: {
  ariaPrefix: string;
  value: unknown;
  onChange: (locale: 'en-US' | 'zh-CN', value: string | undefined) => void;
}) {
  const intl = usePresentationIntl();
  return (
    <Row gutter={[12, 8]}>
      {(['zh-CN', 'en-US'] as const).map((locale) => (
        <Col xs={24} lg={12} key={locale}>
          <Input
            allowClear
            aria-label={`${ariaPrefix} ${locale}`}
            placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })} (${locale})`}
            value={localizedValue(value, locale)}
            onChange={(event) => onChange(locale, event.target.value || undefined)}
          />
        </Col>
      ))}
    </Row>
  );
}

function BooleanOverride({
  ariaLabel,
  disabledTrue,
  inherited,
  value,
  onChange,
}: {
  ariaLabel: string;
  disabledTrue?: boolean;
  inherited: boolean;
  value: boolean | undefined;
  onChange: (value: boolean | undefined) => void;
}) {
  const intl = usePresentationIntl();
  return (
    <Select
      aria-label={ariaLabel}
      className="w-full"
      options={[
        { value: 'inherit', label: intl.formatMessage({ id: 'presentation.visual.inherit' }) },
        {
          value: 'true',
          label: intl.formatMessage({ id: 'presentation.visual.true' }),
          disabled: disabledTrue,
        },
        { value: 'false', label: intl.formatMessage({ id: 'presentation.visual.false' }) },
      ]}
      value={inherited ? 'inherit' : value ? 'true' : 'false'}
      onChange={(next) => onChange(next === 'inherit' ? undefined : next === 'true')}
    />
  );
}

function NumberOverride({
  ariaLabel,
  maximum,
  minimum,
  value,
  onChange,
}: {
  ariaLabel: string;
  maximum: number;
  minimum: number;
  value: number | undefined;
  onChange: (value: number | undefined) => void;
}) {
  const intl = usePresentationIntl();
  return (
    <InputNumber
      aria-label={ariaLabel}
      className="w-full"
      max={maximum}
      min={minimum}
      placeholder={intl.formatMessage({ id: 'presentation.visual.inherit' })}
      value={value}
      onChange={(next) => onChange(typeof next === 'number' ? next : undefined)}
    />
  );
}

function capabilitySurfaceComponents(
  definition: PageCapabilityDefinition,
  field: PageCapabilityField,
  surface: FieldSurface,
) {
  if (definition.definitionVersion !== '2') return field.components;
  return field.surfaceComponents?.find((mapping) => mapping.surface === surface)?.components ?? [];
}

function conditionFields(
  definition: PageCapabilityDefinition,
  surface: Exclude<FieldSurface, 'list'>,
) {
  return definition.fields.filter((field) => field.surfaces.includes(surface));
}

function FieldEditor({
  capability,
  document,
  field,
  surface,
  onChange,
}: {
  capability: PresentationCapability;
  document: PresentationDraftAST;
  field: PageCapabilityField;
  surface: FieldSurface;
  onChange: (document: PresentationDraftAST) => void;
}) {
  const intl = usePresentationIntl();
  const definition = capability.definition;
  const defaults = defaultFields(definition, surface);
  const fallback = defaults.find((item) => item.field === field.id);
  const patch = presentationFieldOverride(document, surface, field.id);
  const components = capabilitySurfaceComponents(definition, field, surface);
  const inherited = (property: string) => !hasOwn(patch, property);
  const propertyValue = (property: string) => patch?.[property];
  const label = localizedValue(field.label, intl.locale.startsWith('zh') ? 'zh-CN' : 'en-US');
  const limitedCapability = isLimitedTablePresentationCapability(definition);
  const canUseCondition =
    !limitedCapability && surface !== 'list' && !(surface === 'form' && field.required);

  return (
    <Card
      size="small"
      title={`${label ?? field.id} (${field.id})`}
      extra={
        <Button
          disabled={!patch}
          size="small"
          onClick={() => onChange(resetPresentationFieldOverrides(document, surface, field.id))}
        >
          {intl.formatMessage({ id: 'presentation.visual.restore.field' })}
        </Button>
      }
    >
      <Space orientation="vertical" size="middle" className="w-full">
        <div>
          <Typography.Text strong>
            {intl.formatMessage({ id: 'presentation.visual.label' })}
          </Typography.Text>
          <LocalizedInputs
            ariaPrefix={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.label' })}`}
            value={propertyValue('label')}
            onChange={(locale, value) =>
              onChange(
                setPresentationFieldLocalizedOverride(
                  document,
                  surface,
                  field.id,
                  'label',
                  locale,
                  value,
                ),
              )
            }
          />
        </div>
        <Row gutter={[12, 12]}>
          {!limitedCapability ? (
            <Col xs={24} md={12} xl={6}>
              <Space orientation="vertical" size={2} className="w-full">
                <Typography.Text>
                  {intl.formatMessage({ id: 'presentation.visual.component' })}
                </Typography.Text>
                <Select
                  allowClear
                  aria-label={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.component' })}`}
                  className="w-full"
                  options={components.map((component) => ({ value: component, label: component }))}
                  placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })}: ${fallback?.component ?? '—'}`}
                  value={stringValue(propertyValue('component'))}
                  onChange={(value: string | undefined) =>
                    onChange(
                      setPresentationFieldOverride(document, surface, field.id, 'component', value),
                    )
                  }
                />
              </Space>
            </Col>
          ) : null}
          <Col xs={24} md={12} xl={6}>
            <Space orientation="vertical" size={2} className="w-full">
              <Typography.Text>
                {intl.formatMessage({ id: 'presentation.visual.order' })}
              </Typography.Text>
              <NumberOverride
                ariaLabel={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.order' })}`}
                maximum={10_000}
                minimum={0}
                value={numberValue(propertyValue('order'))}
                onChange={(value) =>
                  onChange(
                    setPresentationFieldOverride(document, surface, field.id, 'order', value),
                  )
                }
              />
            </Space>
          </Col>
          <Col xs={24} md={12} xl={6}>
            <Space orientation="vertical" size={2} className="w-full">
              <Typography.Text>
                {intl.formatMessage({ id: 'presentation.visual.hidden' })}
              </Typography.Text>
              <BooleanOverride
                ariaLabel={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.hidden' })}`}
                disabledTrue={surface === 'form' && field.required}
                inherited={inherited('hidden')}
                value={booleanValue(propertyValue('hidden'))}
                onChange={(value) =>
                  onChange(
                    setPresentationFieldOverride(document, surface, field.id, 'hidden', value),
                  )
                }
              />
            </Space>
          </Col>
          {surface === 'list' ? (
            <Col xs={24} md={12} xl={6}>
              <Space orientation="vertical" size={2} className="w-full">
                <Typography.Text>
                  {intl.formatMessage({ id: 'presentation.visual.width' })}
                </Typography.Text>
                <NumberOverride
                  ariaLabel={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.width' })}`}
                  maximum={1200}
                  minimum={60}
                  value={numberValue(propertyValue('width'))}
                  onChange={(value) =>
                    onChange(
                      setPresentationFieldOverride(document, surface, field.id, 'width', value),
                    )
                  }
                />
              </Space>
            </Col>
          ) : null}
          {surface === 'form' || surface === 'detail' ? (
            <Col xs={24} md={12} xl={6}>
              <Space orientation="vertical" size={2} className="w-full">
                <Typography.Text>
                  {intl.formatMessage({ id: 'presentation.visual.span' })}
                </Typography.Text>
                <NumberOverride
                  ariaLabel={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.span' })}`}
                  maximum={24}
                  minimum={1}
                  value={numberValue(propertyValue('span'))}
                  onChange={(value) =>
                    onChange(
                      setPresentationFieldOverride(document, surface, field.id, 'span', value),
                    )
                  }
                />
              </Space>
            </Col>
          ) : null}
        </Row>
        {!limitedCapability && (surface === 'search' || surface === 'form') ? (
          <>
            <div>
              <Typography.Text strong>
                {intl.formatMessage({ id: 'presentation.visual.placeholder' })}
              </Typography.Text>
              <LocalizedInputs
                ariaPrefix={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.placeholder' })}`}
                value={propertyValue('placeholder')}
                onChange={(locale, value) =>
                  onChange(
                    setPresentationFieldLocalizedOverride(
                      document,
                      surface,
                      field.id,
                      'placeholder',
                      locale,
                      value,
                    ),
                  )
                }
              />
            </div>
            <div>
              <Typography.Text strong>
                {intl.formatMessage({ id: 'presentation.visual.help' })}
              </Typography.Text>
              <LocalizedInputs
                ariaPrefix={`${field.id} ${intl.formatMessage({ id: 'presentation.visual.help' })}`}
                value={propertyValue('help')}
                onChange={(locale, value) =>
                  onChange(
                    setPresentationFieldLocalizedOverride(
                      document,
                      surface,
                      field.id,
                      'help',
                      locale,
                      value,
                    ),
                  )
                }
              />
            </div>
          </>
        ) : null}
        {canUseCondition ? (
          <div>
            <Typography.Text strong>
              {intl.formatMessage({ id: 'presentation.visual.visibleWhen' })}
            </Typography.Text>
            <PresentationConditionEditor
              condition={conditionValue(propertyValue('visibleWhen'))}
              fields={conditionFields(definition, surface as Exclude<FieldSurface, 'list'>)}
              rawPath={rawFieldPath(document, surface, field.id)}
              onChange={(condition) =>
                onChange(setPresentationFieldCondition(document, surface, field.id, condition))
              }
            />
          </div>
        ) : (
          <Typography.Text type="secondary">
            {limitedCapability
              ? intl.formatMessage({ id: 'presentation.visual.condition.limited.disabled' })
              : surface === 'list'
                ? intl.formatMessage({ id: 'presentation.visual.condition.list.disabled' })
                : intl.formatMessage({ id: 'presentation.visual.condition.required.disabled' })}
          </Typography.Text>
        )}
        {!limitedCapability
          ? inheritedLabel(intl.formatMessage, !patch, fallback?.component)
          : null}
      </Space>
    </Card>
  );
}

function SortEditor({ capability, document, onChange }: PresentationVisualEditorProps) {
  const intl = usePresentationIntl();
  const definition = capability.definition;
  const list = presentationDraftSection(document, 'list');
  const inherited = !hasOwn(list, 'defaultSort');
  const override = Array.isArray(list.defaultSort)
    ? (list.defaultSort.filter(isObject) as unknown as readonly PageSort[])
    : undefined;
  const current = override ?? definition.defaultPresentation.list.defaultSort;
  const sortable = definition.fields.filter((field) => field.sortable);
  const dataSourceID =
    typeof presentationDraftSpec(document).dataSource === 'string'
      ? String(presentationDraftSpec(document).dataSource)
      : definition.defaultPresentation.dataSource;
  const maximum =
    definition.dataSources.find((source) => source.id === dataSourceID)?.maxSortFields ?? 3;
  const setSort = (sort: readonly PageSort[]) =>
    onChange(
      setPresentationSpecOverride(
        document,
        'list',
        'defaultSort',
        sort as unknown as PresentationJSONValue,
      ),
    );

  return (
    <Space orientation="vertical" size="small" className="w-full">
      <Space wrap>
        <Typography.Text strong>
          {intl.formatMessage({ id: 'presentation.visual.defaultSort' })}
        </Typography.Text>
        {inheritedLabel(intl.formatMessage, inherited, JSON.stringify(current))}
        {inherited ? (
          <Button size="small" onClick={() => setSort(current.map((item) => ({ ...item })))}>
            {intl.formatMessage({ id: 'presentation.visual.override' })}
          </Button>
        ) : (
          <Button
            size="small"
            onClick={() =>
              onChange(setPresentationSpecOverride(document, 'list', 'defaultSort', undefined))
            }
          >
            {intl.formatMessage({ id: 'presentation.visual.inherit' })}
          </Button>
        )}
      </Space>
      {!inherited
        ? current.map((sort, index) => (
            <Row gutter={[8, 8]} key={sort.field}>
              <Col xs={24} md={10}>
                <Select
                  aria-label={`${intl.formatMessage({ id: 'presentation.visual.sort.field' })} ${index + 1}`}
                  className="w-full"
                  options={sortable.map((field) => ({ value: field.id, label: field.id }))}
                  value={sort.field}
                  onChange={(field) =>
                    setSort(
                      current.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, field } : item,
                      ),
                    )
                  }
                />
              </Col>
              <Col xs={16} md={10}>
                <Select
                  aria-label={`${intl.formatMessage({ id: 'presentation.visual.sort.direction' })} ${index + 1}`}
                  className="w-full"
                  options={[
                    { value: 'asc', label: 'asc' },
                    { value: 'desc', label: 'desc' },
                  ]}
                  value={sort.direction}
                  onChange={(direction) =>
                    setSort(
                      current.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, direction } : item,
                      ),
                    )
                  }
                />
              </Col>
              <Col xs={8} md={4}>
                <Button
                  danger
                  size="small"
                  onClick={() => setSort(current.filter((_, itemIndex) => itemIndex !== index))}
                >
                  {intl.formatMessage({ id: 'presentation.visual.remove' })}
                </Button>
              </Col>
            </Row>
          ))
        : null}
      {!inherited && current.length < maximum ? (
        <Button
          size="small"
          disabled={!sortable.some((field) => !current.some((sort) => sort.field === field.id))}
          onClick={() => {
            const field = sortable.find((item) => !current.some((sort) => sort.field === item.id));
            if (field) setSort([...current, { field: field.id, direction: 'asc' }]);
          }}
        >
          {intl.formatMessage({ id: 'presentation.visual.sort.add' })}
        </Button>
      ) : null}
    </Space>
  );
}

function SurfaceEditor({
  capability,
  document,
  surface,
  onChange,
}: PresentationVisualEditorProps & { surface: FieldSurface }) {
  const intl = usePresentationIntl();
  const definition = capability.definition;
  const limitedCapability = isLimitedTablePresentationCapability(definition);
  const section = presentationDraftSection(document, surface);
  const fields = definition.fields
    .filter((field) => field.surfaces.includes(surface))
    .sort((left, right) => {
      const defaults = defaultFields(definition, surface);
      const leftOrder = defaults.find((item) => item.field === left.id)?.order ?? 0;
      const rightOrder = defaults.find((item) => item.field === right.id)?.order ?? 0;
      return leftOrder - rightOrder || left.id.localeCompare(right.id);
    });
  const defaultSection = definition.defaultPresentation[surface];
  const spec = presentationDraftSpec(document);
  const dataSourceID =
    !limitedCapability && typeof spec.dataSource === 'string'
      ? spec.dataSource
      : definition.defaultPresentation.dataSource;
  const dataSource = definition.dataSources.find((source) => source.id === dataSourceID);

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {surface === 'list' ? (
        <Card size="small" title={intl.formatMessage({ id: 'presentation.visual.list.settings' })}>
          <Space orientation="vertical" size="middle" className="w-full">
            <Row gutter={[12, 12]}>
              <Col xs={24} md={12} xl={8}>
                <Space orientation="vertical" size={2} className="w-full">
                  <Typography.Text>
                    {intl.formatMessage({ id: 'presentation.visual.density' })}
                  </Typography.Text>
                  <Select
                    allowClear
                    aria-label={intl.formatMessage({ id: 'presentation.visual.density' })}
                    className="w-full"
                    options={(['compact', 'middle', 'large'] as const).map((density) => ({
                      value: density,
                      label: density,
                    }))}
                    placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })}: ${definition.defaultPresentation.list.density}`}
                    value={
                      section.density === 'compact' ||
                      section.density === 'middle' ||
                      section.density === 'large'
                        ? section.density
                        : undefined
                    }
                    onChange={(value: PresentationDensity | undefined) =>
                      onChange(setPresentationSpecOverride(document, 'list', 'density', value))
                    }
                  />
                </Space>
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Space orientation="vertical" size={2} className="w-full">
                  <Typography.Text>
                    {intl.formatMessage({ id: 'presentation.visual.pageSize' })}
                  </Typography.Text>
                  <Select
                    allowClear
                    aria-label={intl.formatMessage({ id: 'presentation.visual.pageSize' })}
                    className="w-full"
                    options={(dataSource?.pageSizeOptions ?? [10, 20, 50, 100])
                      .filter((size) => !dataSource?.maxPageSize || size <= dataSource.maxPageSize)
                      .map((size) => ({ value: size, label: String(size) }))}
                    placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })}: ${definition.defaultPresentation.list.pageSize}`}
                    value={typeof section.pageSize === 'number' ? section.pageSize : undefined}
                    onChange={(value) =>
                      onChange(setPresentationSpecOverride(document, 'list', 'pageSize', value))
                    }
                  />
                </Space>
              </Col>
            </Row>
            {!limitedCapability ? (
              <SortEditor capability={capability} document={document} onChange={onChange} />
            ) : null}
          </Space>
        </Card>
      ) : null}
      {surface === 'search' ? (
        <Card
          size="small"
          title={intl.formatMessage({ id: 'presentation.visual.search.settings' })}
        >
          <Space orientation="vertical" size={2} className="w-full">
            <Typography.Text>
              {intl.formatMessage({ id: 'presentation.visual.collapsed' })}
            </Typography.Text>
            <BooleanOverride
              ariaLabel={intl.formatMessage({ id: 'presentation.visual.collapsed' })}
              inherited={!hasOwn(section, 'collapsedByDefault')}
              value={
                typeof section.collapsedByDefault === 'boolean'
                  ? section.collapsedByDefault
                  : undefined
              }
              onChange={(value) =>
                onChange(
                  setPresentationSpecOverride(document, 'search', 'collapsedByDefault', value),
                )
              }
            />
          </Space>
        </Card>
      ) : null}
      {surface === 'form' || surface === 'detail' ? (
        <Card size="small" title={intl.formatMessage({ id: 'presentation.visual.layout' })}>
          <Space orientation="vertical" size={2} className="w-full">
            <Typography.Text>
              {intl.formatMessage({ id: 'presentation.visual.columns' })}
            </Typography.Text>
            <NumberOverride
              ariaLabel={`${surface} ${intl.formatMessage({ id: 'presentation.visual.columns' })}`}
              maximum={4}
              minimum={1}
              value={typeof section.columns === 'number' ? section.columns : undefined}
              onChange={(value) =>
                onChange(setPresentationSpecOverride(document, surface, 'columns', value))
              }
            />
            {inheritedLabel(
              intl.formatMessage,
              !hasOwn(section, 'columns'),
              'columns' in defaultSection ? defaultSection.columns : undefined,
            )}
          </Space>
        </Card>
      ) : null}
      <Typography.Title level={5}>
        {intl.formatMessage({ id: `presentation.visual.${surface}.fields` })}
      </Typography.Title>
      {fields.map((field) => (
        <FieldEditor
          capability={capability}
          document={document}
          field={field}
          key={field.id}
          surface={surface}
          onChange={onChange}
        />
      ))}
    </Space>
  );
}

function actionDefault(
  definition: PageCapabilityDefinition,
  action: PageCapabilityAction,
): PageActionPresentation | undefined {
  return definition.defaultPresentation.actions.find((item) => item.action === action.id);
}

function ActionEditor({
  action,
  capability,
  document,
  onChange,
}: PresentationVisualEditorProps & { action: PageCapabilityAction }) {
  const intl = usePresentationIntl();
  const fallback = actionDefault(capability.definition, action);
  const patch = presentationActionOverride(document, action.id);
  const propertyValue = (property: string) => patch?.[property];
  const inherited = (property: string) => !hasOwn(patch, property);
  const placement =
    typeof propertyValue('placement') === 'string'
      ? (propertyValue('placement') as PresentationActionPlacement)
      : fallback?.placement;
  const existingCondition = conditionValue(propertyValue('visibleWhen'));

  return (
    <Card
      size="small"
      title={action.id}
      extra={
        <Button
          disabled={!patch}
          size="small"
          onClick={() => onChange(resetPresentationActionOverrides(document, action.id))}
        >
          {intl.formatMessage({ id: 'presentation.visual.restore.action' })}
        </Button>
      }
    >
      <Space orientation="vertical" size="middle" className="w-full">
        <div>
          <Typography.Text strong>
            {intl.formatMessage({ id: 'presentation.visual.label' })}
          </Typography.Text>
          <LocalizedInputs
            ariaPrefix={`${action.id} ${intl.formatMessage({ id: 'presentation.visual.label' })}`}
            value={propertyValue('label')}
            onChange={(locale, value) =>
              onChange(
                setPresentationActionLocalizedOverride(document, action.id, 'label', locale, value),
              )
            }
          />
        </div>
        <Row gutter={[12, 12]}>
          <Col xs={24} md={8}>
            <Space orientation="vertical" size={2} className="w-full">
              <Typography.Text>
                {intl.formatMessage({ id: 'presentation.visual.placement' })}
              </Typography.Text>
              <Select
                allowClear
                aria-label={`${action.id} ${intl.formatMessage({ id: 'presentation.visual.placement' })}`}
                className="w-full"
                options={action.placements.map((item) => ({ value: item, label: item }))}
                placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })}: ${fallback?.placement ?? '—'}`}
                value={
                  action.placements.includes(
                    stringValue(propertyValue('placement')) as PresentationActionPlacement,
                  )
                    ? (propertyValue('placement') as PresentationActionPlacement)
                    : undefined
                }
                onChange={(value: PresentationActionPlacement | undefined) =>
                  onChange(setPresentationActionOverride(document, action.id, 'placement', value))
                }
              />
            </Space>
          </Col>
          <Col xs={24} md={8}>
            <Space orientation="vertical" size={2} className="w-full">
              <Typography.Text>
                {intl.formatMessage({ id: 'presentation.visual.order' })}
              </Typography.Text>
              <NumberOverride
                ariaLabel={`${action.id} ${intl.formatMessage({ id: 'presentation.visual.order' })}`}
                maximum={10_000}
                minimum={0}
                value={numberValue(propertyValue('order'))}
                onChange={(value) =>
                  onChange(setPresentationActionOverride(document, action.id, 'order', value))
                }
              />
            </Space>
          </Col>
          <Col xs={24} md={8}>
            <Space orientation="vertical" size={2} className="w-full">
              <Typography.Text>
                {intl.formatMessage({ id: 'presentation.visual.hidden' })}
              </Typography.Text>
              <BooleanOverride
                ariaLabel={`${action.id} ${intl.formatMessage({ id: 'presentation.visual.hidden' })}`}
                inherited={inherited('hidden')}
                value={booleanValue(propertyValue('hidden'))}
                onChange={(value) =>
                  onChange(setPresentationActionOverride(document, action.id, 'hidden', value))
                }
              />
            </Space>
          </Col>
        </Row>
        <div>
          <Typography.Text strong>
            {intl.formatMessage({ id: 'presentation.visual.confirm' })}
          </Typography.Text>
          <LocalizedInputs
            ariaPrefix={`${action.id} ${intl.formatMessage({ id: 'presentation.visual.confirm' })}`}
            value={propertyValue('confirm')}
            onChange={(locale, value) =>
              onChange(
                setPresentationActionLocalizedOverride(
                  document,
                  action.id,
                  'confirm',
                  locale,
                  value,
                ),
              )
            }
          />
        </div>
        {placement === 'toolbar' ? (
          existingCondition ? (
            <Alert
              description={rawActionPath(document, action.id)}
              title={intl.formatMessage({ id: 'presentation.visual.condition.toolbar.invalid' })}
              type="warning"
            />
          ) : (
            <Typography.Text type="secondary">
              {intl.formatMessage({ id: 'presentation.visual.condition.toolbar.disabled' })}
            </Typography.Text>
          )
        ) : (
          <div>
            <Typography.Text strong>
              {intl.formatMessage({ id: 'presentation.visual.visibleWhen' })}
            </Typography.Text>
            <PresentationConditionEditor
              condition={existingCondition}
              fields={capability.definition.fields}
              rawPath={rawActionPath(document, action.id)}
              onChange={(condition) =>
                onChange(setPresentationActionCondition(document, action.id, condition))
              }
            />
          </div>
        )}
      </Space>
    </Card>
  );
}

function GeneralEditor({ capability, document, onChange }: PresentationVisualEditorProps) {
  const intl = usePresentationIntl();
  const spec = presentationDraftSpec(document);
  const limitedCapability = isLimitedTablePresentationCapability(capability.definition);
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      <Card size="small" title={intl.formatMessage({ id: 'presentation.visual.title' })}>
        <LocalizedInputs
          ariaPrefix={intl.formatMessage({ id: 'presentation.visual.title' })}
          value={spec.title}
          onChange={(locale, value) =>
            onChange(setPresentationLocalizedOverride(document, undefined, 'title', locale, value))
          }
        />
      </Card>
      {!limitedCapability ? (
        <Card size="small" title={intl.formatMessage({ id: 'presentation.visual.dataSource' })}>
          <Select
            allowClear
            aria-label={intl.formatMessage({ id: 'presentation.visual.dataSource' })}
            className="w-full"
            options={capability.definition.dataSources.map((source) => ({
              value: source.id,
              label: source.id,
            }))}
            placeholder={`${intl.formatMessage({ id: 'presentation.visual.inherit' })}: ${capability.definition.defaultPresentation.dataSource}`}
            value={typeof spec.dataSource === 'string' ? spec.dataSource : undefined}
            onChange={(value) =>
              onChange(setPresentationSpecOverride(document, undefined, 'dataSource', value))
            }
          />
        </Card>
      ) : null}
    </Space>
  );
}

export default function PresentationVisualEditor(props: PresentationVisualEditorProps) {
  const intl = usePresentationIntl();
  const { capability, document, onChange } = props;
  const limitedCapability = isLimitedTablePresentationCapability(capability.definition);
  const editableSurfaces = limitedCapability
    ? surfaces.filter((surface) => surface === 'list' || surface === 'search')
    : surfaces;
  return (
    <Tabs
      destroyOnHidden={false}
      items={[
        {
          key: 'general',
          label: intl.formatMessage({ id: 'presentation.visual.general' }),
          children: <GeneralEditor {...props} />,
        },
        ...editableSurfaces.map((surface) => ({
          key: surface,
          label: intl.formatMessage({ id: `presentation.visual.${surface}` }),
          children: (
            <SurfaceEditor
              capability={capability}
              document={document}
              surface={surface}
              onChange={onChange}
            />
          ),
        })),
        ...(limitedCapability
          ? []
          : [
              {
                key: 'actions',
                label: intl.formatMessage({ id: 'presentation.visual.actions' }),
                children: (
                  <Space orientation="vertical" size="middle" className="w-full">
                    {capability.definition.actions.map((action) => (
                      <ActionEditor
                        action={action}
                        capability={capability}
                        document={document}
                        key={action.id}
                        onChange={onChange}
                      />
                    ))}
                  </Space>
                ),
              },
            ]),
      ]}
    />
  );
}
