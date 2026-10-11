export function confirmationMatches(planHash: string, typed: string) {
  return typed.trim().toLowerCase() === planHash.slice(0, 8).toLowerCase();
}
