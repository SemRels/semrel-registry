declare module 'vanilla-cookieconsent' {
  export function run(config: unknown): Promise<void> | void;
  export function acceptedCategory(category: string): boolean;
}
