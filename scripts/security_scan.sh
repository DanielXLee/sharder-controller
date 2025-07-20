#!/bin/bash

# Security Scanning Script for K8s Shard Controller
# Performs comprehensive security validation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCAN_RESULTS_DIR="security_scan_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo -e "${BLUE}=== K8s Shard Controller Security Scan ===${NC}"

# Function to print status
print_status() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')] $1${NC}"
}

print_success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

print_error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

# Create results directory
mkdir -p $SCAN_RESULTS_DIR

# Function to check if tool is available
check_tool() {
    if command -v $1 &> /dev/null; then
        return 0
    else
        return 1
    fi
}

# Scan Kubernetes manifests with kubesec
scan_with_kubesec() {
    print_status "Running kubesec security scan..."
    
    if check_tool kubesec; then
        KUBESEC_RESULTS="$SCAN_RESULTS_DIR/kubesec_results_$TIMESTAMP.json"
        
        # Scan all deployment manifests
        for manifest in manifests/deployment/*.yaml; do
            if [ -f "$manifest" ]; then
                echo "Scanning $manifest..." >> $KUBESEC_RESULTS
                kubesec scan "$manifest" >> $KUBESEC_RESULTS 2>&1 || true
                echo "---" >> $KUBESEC_RESULTS
            fi
        done
        
        # Scan RBAC manifests
        if [ -f "manifests/rbac.yaml" ]; then
            echo "Scanning RBAC manifest..." >> $KUBESEC_RESULTS
            kubesec scan "manifests/rbac.yaml" >> $KUBESEC_RESULTS 2>&1 || true
        fi
        
        print_success "kubesec scan completed - results in $KUBESEC_RESULTS"
    else
        print_warning "kubesec not available - install with: go install github.com/controlplaneio/kubesec/v2/cmd/kubesec@latest"
    fi
}

# Scan container images with trivy
scan_with_trivy() {
    print_status "Running Trivy container security scan..."
    
    if check_tool trivy; then
        TRIVY_RESULTS="$SCAN_RESULTS_DIR/trivy_results_$TIMESTAMP.txt"
        
        # Scan the controller image (if built locally)
        if docker images | grep -q "shard-controller"; then
            echo "Scanning shard-controller image..." > $TRIVY_RESULTS
            trivy image --severity HIGH,CRITICAL shard-controller:latest >> $TRIVY_RESULTS 2>&1 || true
        else
            echo "shard-controller image not found locally" > $TRIVY_RESULTS
        fi
        
        # Scan base images used in Dockerfile
        if [ -f "Dockerfile" ]; then
            BASE_IMAGE=$(grep "^FROM" Dockerfile | head -1 | awk '{print $2}')
            if [ ! -z "$BASE_IMAGE" ]; then
                echo "Scanning base image: $BASE_IMAGE" >> $TRIVY_RESULTS
                trivy image --severity HIGH,CRITICAL "$BASE_IMAGE" >> $TRIVY_RESULTS 2>&1 || true
            fi
        fi
        
        print_success "Trivy scan completed - results in $TRIVY_RESULTS"
    else
        print_warning "trivy not available - install from https://github.com/aquasecurity/trivy"
    fi
}

# Scan Go dependencies for vulnerabilities
scan_go_dependencies() {
    print_status "Scanning Go dependencies for vulnerabilities..."
    
    if check_tool govulncheck; then
        GOVULN_RESULTS="$SCAN_RESULTS_DIR/govulncheck_results_$TIMESTAMP.txt"
        
        echo "Go vulnerability scan results:" > $GOVULN_RESULTS
        echo "Generated: $(date)" >> $GOVULN_RESULTS
        echo "---" >> $GOVULN_RESULTS
        
        govulncheck ./... >> $GOVULN_RESULTS 2>&1 || true
        
        print_success "Go vulnerability scan completed - results in $GOVULN_RESULTS"
    else
        print_warning "govulncheck not available - install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
    fi
}

# Analyze RBAC permissions
analyze_rbac() {
    print_status "Analyzing RBAC permissions..."
    
    RBAC_ANALYSIS="$SCAN_RESULTS_DIR/rbac_analysis_$TIMESTAMP.txt"
    
    echo "RBAC Security Analysis" > $RBAC_ANALYSIS
    echo "Generated: $(date)" >> $RBAC_ANALYSIS
    echo "========================" >> $RBAC_ANALYSIS
    echo "" >> $RBAC_ANALYSIS
    
    if [ -f "manifests/rbac.yaml" ]; then
        echo "Analyzing RBAC manifest..." >> $RBAC_ANALYSIS
        echo "" >> $RBAC_ANALYSIS
        
        # Extract and analyze permissions
        echo "ClusterRole permissions:" >> $RBAC_ANALYSIS
        grep -A 20 "kind: ClusterRole" manifests/rbac.yaml | grep -E "(apiGroups|resources|verbs)" >> $RBAC_ANALYSIS || true
        echo "" >> $RBAC_ANALYSIS
        
        # Check for overly broad permissions
        echo "Security concerns:" >> $RBAC_ANALYSIS
        if grep -q "resources: \[\"*\"\]" manifests/rbac.yaml; then
            echo "- WARNING: Wildcard resource permissions found" >> $RBAC_ANALYSIS
        fi
        if grep -q "verbs: \[\"*\"\]" manifests/rbac.yaml; then
            echo "- WARNING: Wildcard verb permissions found" >> $RBAC_ANALYSIS
        fi
        if grep -q "apiGroups: \[\"*\"\]" manifests/rbac.yaml; then
            echo "- WARNING: Wildcard API group permissions found" >> $RBAC_ANALYSIS
        fi
        
        # Check for cluster-admin binding
        if grep -q "cluster-admin" manifests/rbac.yaml; then
            echo "- CRITICAL: cluster-admin role binding found" >> $RBAC_ANALYSIS
        fi
        
        echo "" >> $RBAC_ANALYSIS
        echo "Recommendations:" >> $RBAC_ANALYSIS
        echo "- Follow principle of least privilege" >> $RBAC_ANALYSIS
        echo "- Use specific resource names where possible" >> $RBAC_ANALYSIS
        echo "- Avoid wildcard permissions" >> $RBAC_ANALYSIS
        echo "- Regular review of permissions" >> $RBAC_ANALYSIS
        
    else
        echo "RBAC manifest not found" >> $RBAC_ANALYSIS
    fi
    
    print_success "RBAC analysis completed - results in $RBAC_ANALYSIS"
}

# Check Dockerfile security best practices
analyze_dockerfile() {
    print_status "Analyzing Dockerfile security..."
    
    DOCKERFILE_ANALYSIS="$SCAN_RESULTS_DIR/dockerfile_analysis_$TIMESTAMP.txt"
    
    echo "Dockerfile Security Analysis" > $DOCKERFILE_ANALYSIS
    echo "Generated: $(date)" >> $DOCKERFILE_ANALYSIS
    echo "============================" >> $DOCKERFILE_ANALYSIS
    echo "" >> $DOCKERFILE_ANALYSIS
    
    if [ -f "Dockerfile" ]; then
        echo "Analyzing Dockerfile..." >> $DOCKERFILE_ANALYSIS
        echo "" >> $DOCKERFILE_ANALYSIS
        
        # Check for security best practices
        echo "Security checks:" >> $DOCKERFILE_ANALYSIS
        
        # Check if running as non-root
        if grep -q "USER" Dockerfile; then
            echo "✓ Non-root user specified" >> $DOCKERFILE_ANALYSIS
        else
            echo "⚠ WARNING: No USER directive found - container may run as root" >> $DOCKERFILE_ANALYSIS
        fi
        
        # Check for COPY vs ADD
        if grep -q "ADD" Dockerfile; then
            echo "⚠ WARNING: ADD instruction found - prefer COPY for security" >> $DOCKERFILE_ANALYSIS
        else
            echo "✓ Using COPY instead of ADD" >> $DOCKERFILE_ANALYSIS
        fi
        
        # Check for specific version tags
        if grep "FROM.*:latest" Dockerfile; then
            echo "⚠ WARNING: Using 'latest' tag - prefer specific versions" >> $DOCKERFILE_ANALYSIS
        else
            echo "✓ Using specific image versions" >> $DOCKERFILE_ANALYSIS
        fi
        
        # Check for secrets in build
        if grep -i -E "(password|secret|key|token)" Dockerfile; then
            echo "⚠ WARNING: Potential secrets found in Dockerfile" >> $DOCKERFILE_ANALYSIS
        else
            echo "✓ No obvious secrets in Dockerfile" >> $DOCKERFILE_ANALYSIS
        fi
        
        # Check for health checks
        if grep -q "HEALTHCHECK" Dockerfile; then
            echo "✓ Health check configured" >> $DOCKERFILE_ANALYSIS
        else
            echo "⚠ INFO: No health check configured" >> $DOCKERFILE_ANALYSIS
        fi
        
        echo "" >> $DOCKERFILE_ANALYSIS
        echo "Recommendations:" >> $DOCKERFILE_ANALYSIS
        echo "- Use specific image tags instead of 'latest'" >> $DOCKERFILE_ANALYSIS
        echo "- Run containers as non-root user" >> $DOCKERFILE_ANALYSIS
        echo "- Use COPY instead of ADD" >> $DOCKERFILE_ANALYSIS
        echo "- Implement health checks" >> $DOCKERFILE_ANALYSIS
        echo "- Use multi-stage builds to reduce attack surface" >> $DOCKERFILE_ANALYSIS
        echo "- Regularly update base images" >> $DOCKERFILE_ANALYSIS
        
    else
        echo "Dockerfile not found" >> $DOCKERFILE_ANALYSIS
    fi
    
    print_success "Dockerfile analysis completed - results in $DOCKERFILE_ANALYSIS"
}

# Check for secrets in code
scan_for_secrets() {
    print_status "Scanning for hardcoded secrets..."
    
    SECRETS_SCAN="$SCAN_RESULTS_DIR/secrets_scan_$TIMESTAMP.txt"
    
    echo "Secrets Scan Results" > $SECRETS_SCAN
    echo "Generated: $(date)" >> $SECRETS_SCAN
    echo "===================" >> $SECRETS_SCAN
    echo "" >> $SECRETS_SCAN
    
    # Common secret patterns
    SECRET_PATTERNS=(
        "password\s*=\s*['\"][^'\"]*['\"]"
        "secret\s*=\s*['\"][^'\"]*['\"]"
        "token\s*=\s*['\"][^'\"]*['\"]"
        "api[_-]?key\s*=\s*['\"][^'\"]*['\"]"
        "private[_-]?key"
        "BEGIN RSA PRIVATE KEY"
        "BEGIN PRIVATE KEY"
        "BEGIN OPENSSH PRIVATE KEY"
    )
    
    echo "Scanning for potential secrets in source code..." >> $SECRETS_SCAN
    echo "" >> $SECRETS_SCAN
    
    SECRETS_FOUND=false
    for pattern in "${SECRET_PATTERNS[@]}"; do
        echo "Checking pattern: $pattern" >> $SECRETS_SCAN
        if grep -r -i -E "$pattern" --include="*.go" --include="*.yaml" --include="*.yml" --exclude-dir=".git" . >> $SECRETS_SCAN 2>/dev/null; then
            SECRETS_FOUND=true
        fi
        echo "" >> $SECRETS_SCAN
    done
    
    if [ "$SECRETS_FOUND" = false ]; then
        echo "✓ No obvious secrets found in source code" >> $SECRETS_SCAN
    else
        echo "⚠ WARNING: Potential secrets found - review above results" >> $SECRETS_SCAN
    fi
    
    print_success "Secrets scan completed - results in $SECRETS_SCAN"
}

# Generate security report
generate_security_report() {
    print_status "Generating comprehensive security report..."
    
    SECURITY_REPORT="$SCAN_RESULTS_DIR/security_report_$TIMESTAMP.md"
    
    cat > $SECURITY_REPORT << EOF
# K8s Shard Controller - Security Scan Report

**Generated:** $(date)
**Scan ID:** $TIMESTAMP

## Executive Summary

This report contains the results of a comprehensive security scan of the K8s Shard Controller project, including:
- Container image vulnerabilities
- Kubernetes manifest security
- RBAC permission analysis
- Dockerfile security best practices
- Source code secret scanning
- Go dependency vulnerabilities

## Scan Results

### Container Security (Trivy)
$(if [ -f "$SCAN_RESULTS_DIR/trivy_results_$TIMESTAMP.txt" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

### Kubernetes Security (kubesec)
$(if [ -f "$SCAN_RESULTS_DIR/kubesec_results_$TIMESTAMP.json" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

### RBAC Analysis
$(if [ -f "$SCAN_RESULTS_DIR/rbac_analysis_$TIMESTAMP.txt" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

### Dockerfile Security
$(if [ -f "$SCAN_RESULTS_DIR/dockerfile_analysis_$TIMESTAMP.txt" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

### Secrets Scanning
$(if [ -f "$SCAN_RESULTS_DIR/secrets_scan_$TIMESTAMP.txt" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

### Go Dependencies
$(if [ -f "$SCAN_RESULTS_DIR/govulncheck_results_$TIMESTAMP.txt" ]; then echo "✓ Completed - see detailed results"; else echo "⚠ Not available"; fi)

## Security Recommendations

### High Priority
1. **Container Security**: Regularly update base images and scan for vulnerabilities
2. **RBAC**: Follow principle of least privilege for service account permissions
3. **Secrets Management**: Use Kubernetes secrets instead of hardcoded values
4. **Network Security**: Implement network policies to restrict pod-to-pod communication

### Medium Priority
1. **Image Scanning**: Integrate container scanning into CI/CD pipeline
2. **Security Policies**: Implement Pod Security Standards
3. **Monitoring**: Set up security monitoring and alerting
4. **Access Control**: Implement proper authentication and authorization

### Low Priority
1. **Documentation**: Maintain security documentation and runbooks
2. **Training**: Regular security training for development team
3. **Auditing**: Regular security audits and penetration testing

## Compliance Considerations

### CIS Kubernetes Benchmark
- [ ] Pod Security Standards implemented
- [ ] Network policies configured
- [ ] RBAC properly configured
- [ ] Secrets management in place

### NIST Cybersecurity Framework
- [ ] Identify: Asset inventory and risk assessment
- [ ] Protect: Access controls and data protection
- [ ] Detect: Security monitoring and logging
- [ ] Respond: Incident response procedures
- [ ] Recover: Backup and recovery procedures

## Next Steps

1. Review all scan results in detail
2. Address high-priority security findings
3. Implement recommended security controls
4. Integrate security scanning into CI/CD pipeline
5. Schedule regular security reviews

## Detailed Results

Detailed scan results are available in the following files:
EOF

    # Add links to detailed results
    for result_file in $SCAN_RESULTS_DIR/*_$TIMESTAMP.*; do
        if [ -f "$result_file" ]; then
            echo "- [$(basename $result_file)]($result_file)" >> $SECURITY_REPORT
        fi
    done
    
    cat >> $SECURITY_REPORT << EOF

---
*This report was generated automatically. Review all findings and take appropriate action based on your security requirements and risk tolerance.*
EOF

    print_success "Security report generated: $SECURITY_REPORT"
}

# Main execution
main() {
    print_status "Starting comprehensive security scan..."
    
    # Run all security scans
    scan_with_kubesec
    scan_with_trivy
    scan_go_dependencies
    analyze_rbac
    analyze_dockerfile
    scan_for_secrets
    
    # Generate comprehensive report
    generate_security_report
    
    print_success "Security scan completed!"
    print_status "Review the generated reports in $SCAN_RESULTS_DIR/"
    
    # Summary
    echo ""
    echo -e "${BLUE}=== Security Scan Summary ===${NC}"
    echo "Results directory: $SCAN_RESULTS_DIR/"
    echo "Main report: $SCAN_RESULTS_DIR/security_report_$TIMESTAMP.md"
    echo ""
    echo "Next steps:"
    echo "1. Review all scan results"
    echo "2. Address critical and high-severity findings"
    echo "3. Implement security recommendations"
    echo "4. Integrate scanning into CI/CD pipeline"
}

# Run main function
main