const fail = () => { throw new TypeError("hostile value") }
export const primitive = { [Symbol.toPrimitive]: fail }
export const getter = { get message(): string { return fail() } }
const revocable = Proxy.revocable({}, {})
revocable.revoke()
export const revoked = revocable.proxy
export const prototype = new Proxy({}, { getPrototypeOf: fail })
