export const getMenuLocaleId = (name?: string): string => {
  const normalizedName = name?.trim();
  if (!normalizedName) {
    return '';
  }

  const localeKeyOffset = normalizedName.lastIndexOf('menu.');
  return localeKeyOffset >= 0 ? normalizedName.slice(localeKeyOffset) : `menu.${normalizedName}`;
};
