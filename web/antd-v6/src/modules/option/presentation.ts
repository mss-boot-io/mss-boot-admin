import type { OptionDetail, OptionItem, OptionSummary } from './contract';

interface MessageFormatter {
  formatMessage(descriptor: { id: string }): string;
}

interface BuiltInOptionPresentation {
  description: readonly [source: string, messageID: string];
  displayName: readonly [source: string, messageID: string];
  itemLabels: Readonly<Record<string, readonly [source: string, messageID: string]>>;
  remark: readonly [source: string, messageID: string];
}

const BUILT_IN_PRESENTATIONS: Readonly<Record<string, BuiltInOptionPresentation>> = {
  'system/status': {
    displayName: ['Status Options', 'option.builtIn.status.displayName'],
    description: ['Basic status options for all entities', 'option.builtIn.status.description'],
    remark: ['System status options', 'option.builtIn.status.remark'],
    itemLabels: {
      enabled: ['Enabled', 'option.builtIn.status.item.enabled'],
      disabled: ['Disabled', 'option.builtIn.status.item.disabled'],
      locked: ['Locked', 'option.builtIn.status.item.locked'],
    },
  },
  'permission/dataScope': {
    displayName: ['Data Scope Options', 'option.builtIn.dataScope.displayName'],
    description: [
      'Data scope options for permission control',
      'option.builtIn.dataScope.description',
    ],
    remark: ['System data scope options', 'option.builtIn.dataScope.remark'],
    itemLabels: {
      all: ['All', 'option.builtIn.dataScope.item.all'],
      currentDept: ['Current Department', 'option.builtIn.dataScope.item.currentDepartment'],
      currentAndChildrenDept: [
        'Current and Children Departments',
        'option.builtIn.dataScope.item.currentAndChildrenDepartments',
      ],
      customDept: ['Custom Departments', 'option.builtIn.dataScope.item.customDepartments'],
      self: ['Self Only', 'option.builtIn.dataScope.item.selfOnly'],
      selfAndChildren: ['Self and Children', 'option.builtIn.dataScope.item.selfAndChildren'],
      selfAndAllChildren: [
        'Self and All Children',
        'option.builtIn.dataScope.item.selfAndAllChildren',
      ],
    },
  },
};

function definitionFor(option: Pick<OptionSummary, 'builtIn' | 'category' | 'name'>) {
  return option.builtIn ? BUILT_IN_PRESENTATIONS[`${option.category}/${option.name}`] : undefined;
}

function translatedSeed(
  value: string,
  seed: readonly [source: string, messageID: string] | undefined,
  intl: MessageFormatter,
): string {
  return seed && value === seed[0] ? intl.formatMessage({ id: seed[1] }) : value;
}

export function presentOptionSummary<T extends OptionSummary>(
  option: T,
  intl: MessageFormatter,
): T {
  const definition = definitionFor(option);
  if (!definition) return option;
  return {
    ...option,
    displayName: translatedSeed(option.displayName, definition.displayName, intl),
    remark: translatedSeed(option.remark, definition.remark, intl),
  };
}

function presentItem(
  item: OptionItem,
  definition: BuiltInOptionPresentation,
  intl: MessageFormatter,
): OptionItem {
  return {
    ...item,
    label: translatedSeed(item.label, definition.itemLabels[item.key], intl),
  };
}

export function presentOptionDetail(option: OptionDetail, intl: MessageFormatter): OptionDetail {
  const definition = definitionFor(option);
  if (!definition) return option;
  const summary = presentOptionSummary(option, intl);
  return {
    ...summary,
    description: translatedSeed(option.description, definition.description, intl),
    items: option.items.map((item) => presentItem(item, definition, intl)),
  };
}
