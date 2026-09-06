import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import tseslint from 'typescript-eslint';
import globals from 'globals';

export default tseslint.config(
	{
		// .svelte.ts runes modules are type-checked by svelte-check, not eslint.
		ignores: [
			'**/.svelte-kit/**',
			'**/node_modules/**',
			'**/build/**',
			'backend/**',
			'docs/**',
			'e2e/**',
			'src/**/*.svelte.ts'
		]
	},
	js.configs.recommended,
	...tseslint.configs.recommended,
	...svelte.configs['flat/recommended'],
	{
		files: ['**/*.svelte'],
		languageOptions: {
			parserOptions: { parser: tseslint.parser }
		}
	},
	{
		files: ['**/*.ts', '**/*.svelte'],
		languageOptions: {
			globals: { ...globals.browser, ...globals.node }
		}
	}
);
