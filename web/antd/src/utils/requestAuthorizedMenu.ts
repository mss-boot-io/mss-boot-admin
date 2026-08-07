import { getMenuAuthorize } from '@/services/admin/menu';

/**
 * Keeps ProLayout's runtime menu source explicit and independently testable.
 * The authorized backend response remains authoritative and is returned
 * without filtering or reshaping, so unrelated dynamic menus keep working.
 */
export const requestAuthorizedMenu = () => getMenuAuthorize();

export default requestAuthorizedMenu;
