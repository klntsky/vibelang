package checker

// SignatureWithReturnType returns signature's type parameters and parameters
// with returnType as the return type, so a language service can show a
// function the way a language built on the checker calls it.
func (c *Checker) SignatureWithReturnType(signature *Signature, returnType *Type) *Signature {
	return c.newSignature(signature.flags, nil /*declaration*/, signature.TypeParameters(), signature.ThisParameter(), signature.parameters, returnType, nil /*resolvedTypePredicate*/, int(signature.minArgumentCount))
}
