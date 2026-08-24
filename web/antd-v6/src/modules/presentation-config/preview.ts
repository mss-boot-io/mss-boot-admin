import {
  type AdminLocale,
  type AdminPagePresentationProfile,
  buildPageRenderModel,
  type PageRenderModel,
  resolvePagePresentation,
} from '@mss-admin-core/shared/presentation/contract';
import {
  type PresentationCapability,
  PresentationContractError,
  type PresentationJSONObject,
} from './contract';

export function buildPresentationPreview(
  capability: PresentationCapability,
  document: PresentationJSONObject,
  locale: AdminLocale,
): PageRenderModel {
  const profile = document as unknown as AdminPagePresentationProfile;
  const layer = profile.metadata?.scope?.kind;
  if (layer !== 'application' && layer !== 'role' && layer !== 'user') {
    throw new PresentationContractError('Validated presentation scope is unavailable');
  }
  const permissions = new Set(
    [
      ...capability.definition.dataSources.flatMap((item) => item.requiredPermissions),
      ...capability.definition.actions.flatMap((item) => item.requiredPermissions),
    ].filter(Boolean),
  );
  const resolution = resolvePagePresentation(
    capability.definition,
    { [layer]: profile },
    permissions,
  );
  if (resolution.rejectedLayers.length > 0) {
    throw new PresentationContractError('Validated presentation preview was rejected');
  }
  return buildPageRenderModel(capability.definition, resolution, locale);
}
