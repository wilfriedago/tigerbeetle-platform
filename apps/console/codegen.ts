import { type CodegenConfig } from '@graphql-codegen/cli'

// graphql-codegen writes the typed documents of the console from the schema the API generates
// (ADR-0023, plan section 17.2). The documents live next to their query options in each slice's
// api.ts. documentMode 'string' keeps the graphql package out of the bundle: a document is a typed
// string the client posts as is.
const config: CodegenConfig = {
  schema: '../../packages/graphql/schema.graphql',
  documents: ['src/**/*.ts', '!src/graphql/gen/**'],
  ignoreNoDocuments: true,
  generates: {
    'src/graphql/gen/': {
      preset: 'client',
      presetConfig: { fragmentMasking: false },
      config: {
        documentMode: 'string',
        enumsAsTypes: true,
        useTypeImports: true,
        strictScalars: true,
        // Money and IDs stay strings, never numbers (ADR-0005); Int64 is for counts only.
        scalars: {
          Amount: 'string',
          SignedAmount: 'string',
          U128: 'string',
          U64: 'string',
          DateTime: 'string',
          Int64: 'number',
          Metadata: 'Record<string, string>',
          JSON: 'Record<string, unknown>'
        }
      }
    }
  }
}

export default config
