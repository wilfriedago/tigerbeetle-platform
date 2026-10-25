import tanstackQuery from '@tanstack/eslint-plugin-query'
import tanstackRouter from '@tanstack/eslint-plugin-router'

import { defineConfig } from 'vite-plus'

export default defineConfig({
  fmt: {
    ignorePatterns: ['packages/api-client/src/schema.d.ts'],
    printWidth: 150,
    semi: false,
    singleQuote: true,
    sortImports: {
      customGroups: [
        { elementNamePattern: ['react', 'react-native', 'react-dom/*'], groupName: 'react' },
        { elementNamePattern: ['@tanstack/**'], groupName: 'tanstack' },
        { elementNamePattern: ['~/components/**'], groupName: 'components' },
        { elementNamePattern: ['~/features/**'], groupName: 'features' }
      ],
      groups: [
        'side_effect',
        'type-import',
        'value-builtin',
        'react',
        'tanstack',
        ['type-external', 'value-external'],
        'components',
        'features',
        ['type-internal', 'value-internal'],
        ['value-parent', 'value-sibling', 'value-index', 'type-parent', 'type-sibling', 'type-index'],
        'unknown'
      ]
    },
    sortPackageJson: true,
    trailingComma: 'none'
  },

  lint: {
    categories: {
      correctness: 'error',
      perf: 'warn',
      style: 'warn',
      suspicious: 'warn'
    },
    ignorePatterns: ['packages/api-client/src/schema.d.ts'],
    jsPlugins: [
      { name: 'vite-plus', specifier: 'vite-plus/oxlint-plugin' },
      { name: 'react-doctor', specifier: 'oxlint-plugin-react-doctor' },
      { name: '@tanstack/query', specifier: '@tanstack/eslint-plugin-query' },
      { name: '@tanstack/router', specifier: '@tanstack/eslint-plugin-router' }
    ],
    options: {
      reportUnusedDisableDirectives: 'error',
      typeAware: true,
      typeCheck: true
    },
    overrides: [
      {
        files: ['**/*.test.{ts,tsx}', '**/tests/**/*.{ts,tsx}'],
        plugins: ['vitest'],
        rules: {
          'init-declarations': 'off',
          'prefer-destructuring': 'off',
          'prefer-named-capture-group': 'off',
          'promise/avoid-new': 'off',
          'react/jsx-no-constructed-context-values': 'off',
          'unicorn/no-await-expression-member': 'off',
          'vitest/consistent-test-filename': ['error', { pattern: String.raw`.*\.(test|spec)\.[tj]sx?$` }],
          'vitest/expect-expect': ['error', { assertFunctionNames: ['expect', 'expectNoA11yViolations'] }],
          'vitest/max-expects': 'off',
          'vitest/no-hooks': 'off',
          'vitest/padding-around-after-all-blocks': 'off',
          'vitest/prefer-called-once': 'off',
          'vitest/prefer-called-times': 'off',
          'vitest/prefer-comparison-matcher': 'off',
          'vitest/prefer-describe-function-title': 'off',
          'vitest/prefer-expect-assertions': 'off',
          'vitest/prefer-expect-resolves': 'off',
          'vitest/prefer-hooks-in-order': 'off',
          'vitest/prefer-importing-vitest-globals': 'off',
          'vitest/prefer-lowercase-title': 'off',
          'vitest/prefer-strict-boolean-matchers': 'off',
          'vitest/prefer-strict-equal': 'off',
          'vitest/prefer-to-be-falsy': 'off',
          'vitest/prefer-to-be-truthy': 'off',
          'vitest/require-hook': 'off',
          'vitest/require-top-level-describe': 'off'
        }
      }
    ],
    plugins: ['eslint', 'typescript', 'unicorn', 'oxc', 'react', 'react-perf', 'jsx-a11y', 'import', 'promise', 'vitest'],
    rules: {
      'no-console': 'warn',
      'no-continue': 'off',
      'no-debugger': 'error',
      'one-var': 'off',
      'sort-keys': 'off',
      'sort-imports': [
        'error',
        {
          ignoreCase: true,
          ignoreDeclarationSort: true
        }
      ],
      'capitalized-comments': 'off',
      'max-statements': 'off',
      'func-style': 'off',
      'no-magic-numbers': 'off',
      'no-ternary': 'off',
      'no-nested-ternary': 'off',
      'max-params': 'off',
      'id-length': 'off',
      'init-declarations': 'off',
      'no-underscore-dangle': 'off',
      'no-duplicate-imports': 'off',
      'no-use-before-define': [
        'error',
        {
          allowNamedExports: false,
          classes: true,
          enums: true,
          functions: true,
          ignoreTypeReferences: true,
          typedefs: false,
          variables: true
        }
      ],
      'no-await-in-loop': 'off',
      'no-param-reassign': [
        'error',
        {
          props: false
        }
      ],
      'prefer-arrow-callback': [
        'warn',
        {
          allowNamedFunctions: true
        }
      ],
      'no-restricted-imports': [
        'error',
        {
          paths: [
            { name: 'tailwind-variants', importNames: ['tv', 'createTV'] },
            { name: 'cn', importNames: ['cn'] }
          ],
          patterns: [{ group: ['**/index', './index', '../index'] }]
        }
      ],
      ...tanstackRouter.configs.recommended.rules,
      ...tanstackQuery.configs.recommended.rules,
      '@tanstack/query/exhaustive-deps': 'error',
      '@tanstack/query/no-rest-destructuring': 'warn',
      '@tanstack/query/stable-query-client': 'error',
      '@tanstack/query/no-unstable-deps': 'error',
      '@tanstack/query/infinite-query-property-order': 'error',
      '@tanstack/query/no-void-query-fn': 'error',
      '@tanstack/router/create-route-property-order': 'error',
      'jsx-a11y/prefer-tag-over-role': 'off',
      'react-doctor/no-derived-state': 'warn',
      'react-doctor/no-derived-state-effect': 'warn',
      'react-doctor/no-fetch-in-effect': 'warn',
      'react-perf/jsx-no-jsx-as-prop': 'off',
      'react-perf/jsx-no-new-array-as-prop': 'off',
      'react-perf/jsx-no-new-function-as-prop': 'off',
      'react-perf/jsx-no-new-object-as-prop': 'off',
      'react/error-boundaries': 'error',
      'react/globals': 'error',
      'react/hook-use-state': 'off',
      'react/immutability': 'warn',
      'react/incompatible-library': 'off',
      'react/jsx-handler-names': 'off',
      'react/jsx-max-depth': ['warn', { max: 20 }],
      'react/jsx-props-no-spreading': 'off',
      'react/no-set-state': 'off',
      'react/no-unstable-nested-components': ['error', { allowAsProps: true }],
      'react/preserve-manual-memoization': 'error',
      'react/purity': 'error',
      'react/react-in-jsx-scope': 'off',
      'react/refs': 'warn',
      'react/set-state-in-effect': 'warn',
      'react/set-state-in-render': 'error',
      'react/state-in-constructor': 'off',
      'react/static-components': 'warn',
      'react/style-prop-object': ['warn', { allow: ['StatusBar'] }],
      'react/unsupported-syntax': 'error',
      'react/use-memo': 'error',
      'typescript/consistent-return': 'off',
      'typescript/consistent-type-imports': ['warn', { disallowTypeAnnotations: true, fixStyle: 'inline-type-imports', prefer: 'type-imports' }],
      'typescript/no-unsafe-type-assertion': 'off',
      'typescript/no-unused-vars': ['warn', { argsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'vite-plus/prefer-vite-plus-imports': 'error',
      'promise/prefer-await-to-then': 'off',
      'promise/always-return': ['warn', { ignoreLastCallback: true }],
      'import/no-namespace': 'off',
      'import/exports-last': 'off',
      'import/group-exports': 'off',
      'import/no-named-export': 'off',
      'import/no-unassigned-import': 'off',
      'import/prefer-default-export': 'off',
      'import/consistent-type-specifier-style': ['warn', 'prefer-inline'],
      'import/no-nodejs-modules': ['error', { allow: ['node:path', 'node:fs', 'node:url'] }],
      'unicorn/number-literal-case': 'off',
      'unicorn/no-null': 'off',
      'unicorn/no-array-reduce': 'off',
      'unicorn/max-nested-calls': 'off',
      'unicorn/no-useless-spread': 'warn',
      'unicorn/no-nested-ternary': 'off',
      'unicorn/consistent-function-scoping': 'off',
      'unicorn/filename-case': [
        'error',
        {
          cases: {
            kebabCase: true,
            snakeCase: true
          },
          ignore: [String.raw`^\$`]
        }
      ],
      'node/callback-return': 'off',
      'oxc/no-map-spread': 'off'
    }
  },

  staged: {
    '*.{js,jsx,ts,tsx,mjs,cjs,json,jsonc,css,html,md,yaml,yml}': 'vp check --fix',
    'apps/api/**/*.go': [(files: readonly string[]) => `pnpm --filter api exec golangci-lint fmt ${files.join(' ')}`, () => 'pnpm --filter api lint'],
    'packages/openapi/**/*.yaml': () => 'pnpm --filter @tigerbeetle-platform/openapi lint'
  },

  test: {
    clearMocks: false,
    maxWorkers: '50%',
    projects: ['packages/*/vite.config.ts', 'apps/*/vite.config.ts']
  }
})
