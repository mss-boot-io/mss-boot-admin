'use strict';

const functionToString = Function.prototype.toString;

function isValue(value) {
  return value !== null && value !== undefined;
}

function isPlainFunction(value) {
  return typeof value === 'function' && !/^\s*class[\s{/}]/.test(functionToString.call(value));
}

function mergeOptions(options, descriptor) {
  return options ? Object.assign(Object.create(null), Object(options), descriptor) : descriptor;
}

function descriptor(specification, value, options) {
  let spec = specification;
  let resolvedValue = value;
  let resolvedOptions = options;
  // biome-ignore lint/complexity/noArguments: the compatibility API distinguishes d(value) from d(spec, undefined).
  if (arguments.length < 2 || typeof spec !== 'string') {
    resolvedOptions = value;
    resolvedValue = spec;
    spec = null;
  }
  const configurable = spec ? spec.includes('c') : true;
  const enumerable = spec ? spec.includes('e') : false;
  const writable = spec ? spec.includes('w') : true;
  return mergeOptions(resolvedOptions, {
    value: resolvedValue,
    configurable,
    enumerable,
    writable,
  });
}

descriptor.gs = function getterSetterDescriptor(specification, get, set, options) {
  let spec = specification;
  let resolvedGet = get;
  let resolvedSet = set;
  let resolvedOptions = options;
  if (typeof spec !== 'string') {
    resolvedOptions = set;
    resolvedSet = get;
    resolvedGet = spec;
    spec = null;
  }
  if (!isValue(resolvedGet)) {
    resolvedGet = undefined;
  } else if (!isPlainFunction(resolvedGet)) {
    resolvedOptions = resolvedGet;
    resolvedGet = undefined;
    resolvedSet = undefined;
  } else if (!isValue(resolvedSet)) {
    resolvedSet = undefined;
  } else if (!isPlainFunction(resolvedSet)) {
    resolvedOptions = resolvedSet;
    resolvedSet = undefined;
  }
  return mergeOptions(resolvedOptions, {
    get: resolvedGet,
    set: resolvedSet,
    configurable: spec ? spec.includes('c') : true,
    enumerable: spec ? spec.includes('e') : false,
  });
};

module.exports = descriptor;
