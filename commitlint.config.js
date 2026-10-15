export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // Dependency-bump bodies carry long release-note URLs; warn instead of failing on them.
    'body-max-line-length': [1, 'always', 100],
    'footer-max-line-length': [1, 'always', 100]
  }
}
