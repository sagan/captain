export interface UpdateSignature { hasAction: boolean }
export function planUpdate(prev: UpdateSignature | undefined, option: object): {
  signature: UpdateSignature
  plan: { notMerge: boolean, replaceMerge?: string[] }
}
