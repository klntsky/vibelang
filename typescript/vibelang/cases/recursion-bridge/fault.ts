export const defect = new TypeError("bridge defect")
export const fault = { get value(): number { throw defect } }
