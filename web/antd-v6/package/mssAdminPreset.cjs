module.exports = function mssAdminPreset() {
  return {
    plugins: [require.resolve('./mssAdminPlugin.cjs')],
  };
};
