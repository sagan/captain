// Browser WebAuthn bridge. The credential private key remains in the authenticator.
import { api } from './api'

type DescriptorJSON = Omit<PublicKeyCredentialDescriptor, 'id'> & { id: string }
type CreationJSON = Omit<PublicKeyCredentialCreationOptions, 'challenge' | 'user' | 'excludeCredentials'> & { challenge: string; user: Omit<PublicKeyCredentialUserEntity, 'id'> & { id: string }; excludeCredentials?: DescriptorJSON[] }
type RequestJSON = Omit<PublicKeyCredentialRequestOptions, 'challenge' | 'allowCredentials'> & { challenge: string; allowCredentials?: DescriptorJSON[] }
const decode = (s: string): ArrayBuffer => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)).buffer
const encode = (b: ArrayBuffer) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
function serialize(c: PublicKeyCredential) {
  const response = c.response
  const common = { clientDataJSON: encode(response.clientDataJSON) }
  const data = response instanceof AuthenticatorAttestationResponse
    ? { ...common, attestationObject: encode(response.attestationObject), transports: response.getTransports?.() ?? [] }
    : (() => { const a = response as AuthenticatorAssertionResponse; return { ...common, authenticatorData: encode(a.authenticatorData), signature: encode(a.signature), userHandle: a.userHandle ? encode(a.userHandle) : null } })()
  return { id: c.id, rawId: encode(c.rawId), type: c.type, authenticatorAttachment: c.authenticatorAttachment, clientExtensionResults: c.getClientExtensionResults(), response: data }
}
export const passkeysAvailable = () => typeof window.PublicKeyCredential !== 'undefined' && window.isSecureContext
export async function registerPasskey(name: string, password: string, code: string) {
  const { publicKey } = await api.post<{ publicKey: CreationJSON }>('/api/admin/passkeys/register/begin', { name, password, code })
  const credential = await navigator.credentials.create({ publicKey: { ...publicKey, challenge: decode(publicKey.challenge), user: { ...publicKey.user, id: decode(publicKey.user.id) }, excludeCredentials: publicKey.excludeCredentials?.map(c => ({ ...c, id: decode(c.id) })) } }) as PublicKeyCredential | null
  if (!credential) throw new Error('Passkey creation canceled')
  return api.post('/api/admin/passkeys/register/finish', { credential: serialize(credential) })
}
export async function loginWithPasskey(code: string) {
  const { publicKey } = await api.post<{ publicKey: RequestJSON }>('/api/admin/passkeys/login/begin')
  const credential = await navigator.credentials.get({ publicKey: { ...publicKey, challenge: decode(publicKey.challenge), allowCredentials: publicKey.allowCredentials?.map(c => ({ ...c, id: decode(c.id) })) } }) as PublicKeyCredential | null
  if (!credential) throw new Error('Passkey login canceled')
  return api.post('/api/admin/passkeys/login/finish', { credential: serialize(credential), code })
}
