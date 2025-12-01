# Security Overview

Mistokenly is designed with security as the foundation. Here's what you need to know:

## 🔐 How We Protect Your Data

### Zero-Knowledge Architecture
- **We cannot read your data**. Only you have the keys to decrypt it.
- **Two keys required**: Our platform key + your organization key
- **Cryptographic isolation**: Each organization is completely separate

### Military-Grade Encryption
- **AES-256-GCM**: Industry-standard encryption
- **HKDF key derivation**: Secure key generation
- **Random IVs**: Each encryption uses unique initialization vectors

### Key Management
- **KEK (Platform Key)**: Protects all tenant keys, stored securely
- **TEK (Tenant Key)**: One per organization, encrypted with KEK
- **Derived Keys**: Generated on-demand, never stored

## 🛡️ Security Features

### Data Protection
- ✅ Encrypted at rest and in transit
- ✅ Organization-level isolation
- ✅ Audit logging for all access
- ✅ Secure key generation and storage

### Infrastructure Security
- ✅ Kubernetes deployment with security best practices
- ✅ Service mesh (Linkerd) for encrypted inter-service communication
- ✅ Database encryption and access controls
- ✅ Container security and image scanning

### Access Control
- ✅ Organization-based access control
- ✅ Client ID tracking
- ✅ Request validation and sanitization
- ✅ Least privilege principles

## 🚨 Security Best Practices

### For Organizations Using Mistokenly

1. **Use Strong Organization Keys**
   - Generate 32+ character random keys
   - Store securely (never in code or logs)
   - Use different keys for dev/staging/production

2. **Secure Key Storage**
   - Use KMS, HashiCorp Vault, or similar
   - Never share keys via email or chat
   - Rotate keys regularly (quarterly minimum)

3. **Monitor Access**
   - Review audit logs regularly
   - Alert on suspicious activity
   - Track who accesses what data

### For Platform Operators

1. **Secure KEK Management**
   - Use external KMS (AWS KMS, Azure Key Vault, etc.)
   - Never store KEK in environment variables
   - Implement KEK rotation procedures

2. **Network Security**
   - Enable TLS everywhere
   - Use network policies to restrict traffic
   - Implement zero-trust architecture

3. **Infrastructure Hardening**
   - Keep all software updated
   - Use security groups and firewalls
   - Implement monitoring and alerting

## 🚨 Reporting Security Issues

**Never report security vulnerabilities through public GitHub issues.**

### How to Report
Email: **hello@plainfunction.com.au**

Include:
- Type of issue (e.g., "encryption weakness", "access control bypass")
- Steps to reproduce
- Potential impact
- Your contact information

### What Happens Next
- **1 week**: Acknowlegement and Initial assessment
- **2-4 weeks**: Fix development and testing
- **Public disclosure**: Coordinated with you after fix

## ⚠️ Known Limitations

### Current Considerations
1. **KEK Storage**: Currently uses Kubernetes secrets (use external KMS in production)
2. **Key Rotation**: Manual process (automated rotation planned)
3. **Memory Security**: Keys cached in memory (acceptable risk with proper infrastructure)

### Risk Mitigation
- Defense in depth with multiple encryption layers
- Zero-knowledge design prevents platform data access
- Comprehensive audit logging
- Regular security updates

## 📋 Compliance

Mistokenly helps support compliance with:
- **GDPR**: Data protection and privacy rights
- **HIPAA**: Protected health information (with proper configuration)
- **PCI DSS**: Payment card data tokenization
- **SOC 2**: Security, availability, and confidentiality controls

## 📞 Contact

- **General Support**: hello@plainfunction.com.au

---

**Last Updated**: November 2025
