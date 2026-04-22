# Fault Studio Integration with Experiment Builder

## Business Requirements Document

**Document Owner:** Product Management  
**Target Audience:** Business Stakeholders, Leadership  
**Date:** February 2026

---

## Executive Summary

This document outlines the business case for integrating **Fault Studio** with the **Experiment Builder** in our Chaos Engineering platform. This enhancement enables users—particularly AI Agents—to rapidly create chaos experiments using pre-configured fault packages, reducing experiment setup time from minutes to seconds.

---

## 1. What is ChaosHub Today?

**ChaosHub** is our centralized library of chaos faults—think of it as an "app store" for chaos testing capabilities. 

Today, ChaosHub provides:

- **A catalog of 50+ ready-to-use faults** such as killing pods, injecting network delays, consuming CPU/memory, and corrupting data
- **Organized categories** like Network, CPU, Memory, Disk, and Application-level faults
- **Default configurations** that work out-of-the-box for common scenarios
- **The ability to add custom hubs** for organization-specific fault definitions

**Current User Experience:**

When a user wants to create a chaos experiment today, they must:
1. Open the Experiment Builder
2. Click "Add Fault"
3. Browse through the ChaosHub catalog
4. Select a fault (e.g., "pod-network-latency")
5. Manually configure the fault settings (duration, target application, latency amount, etc.)
6. Repeat steps 2-5 for each additional fault they want to test

**The Problem:**

While ChaosHub provides a rich library of faults, users must configure each fault from scratch every time they create an experiment. This is time-consuming and error-prone, especially when:
- The same fault configurations are used repeatedly across multiple experiments
- Teams want to standardize their chaos testing practices
- AI Agents need to programmatically create experiments without detailed configuration knowledge

---

## 2. Why Integrate Fault Studio with Experiments?

### The Business Need

**Fault Studio** was introduced to allow users to create curated collections of pre-configured faults. Think of it as creating "playlists" of chaos faults with all the settings already dialed in.

However, Fault Studio currently exists as a standalone feature. Users can create and manage their fault collections, but they cannot directly use these collections when building experiments. This creates a disconnect in the user workflow.

### The AI Agent Use Case

Our platform is evolving to support **AI-powered chaos engineering**, where intelligent agents can:
- Analyze application architecture and recommend appropriate chaos tests
- Automatically generate experiments based on resilience requirements
- Execute experiments as part of CI/CD pipelines without human intervention

**For AI Agents to work effectively, they need:**

1. **Pre-packaged fault configurations** – Instead of understanding the intricacies of each fault's 15+ configurable parameters, an AI Agent can simply say "use the Network Resilience package" and get a proven set of network faults with appropriate settings.

2. **Consistent, repeatable experiments** – When an AI Agent creates an experiment for "testing payment service resilience," it should use the same fault configurations every time, ensuring comparable results.

3. **Reduced decision complexity** – An AI Agent shouldn't need to decide whether network latency should be 100ms or 200ms, or whether to target pods by label or by name. These decisions should be pre-made by human experts and packaged into Fault Studios.

### Business Benefits

| Benefit | Impact |
|---------|--------|
| **Faster Experiment Creation** | Reduce setup time from 10-15 minutes to under 1 minute |
| **Standardization** | Ensure all teams use approved, tested fault configurations |
| **AI Enablement** | Allow AI Agents to create sophisticated experiments without deep configuration knowledge |
| **Reduced Errors** | Eliminate manual configuration mistakes by reusing proven settings |
| **Knowledge Capture** | Expert configurations are preserved in studios, not lost when team members leave |

---

## 3. How Will It Work After Integration?

### The New User Journey

**For Human Users:**

1. User opens the Experiment Builder to create a new chaos experiment
2. User clicks "Add Fault" button
3. A selection panel appears with **two tabs**:
   - **ChaosHub** (existing) – Browse and configure faults from scratch
   - **Fault Studios** (new) – Select pre-configured faults from saved collections
4. User clicks the "Fault Studios" tab
5. User sees their available studios:
   - "Network Resilience Tests" (5 faults ready)
   - "Database Stress Package" (3 faults ready)
   - "Payment Service Chaos" (4 faults ready)
6. User expands "Network Resilience Tests" and sees:
   - ✓ Pod Network Latency – 200ms delay, targets frontend pods
   - ✓ Pod Network Loss – 30% packet loss, 60-second duration
   - ✓ DNS Failure – Simulates DNS resolution failures
7. User selects the faults they want (multi-select supported)
8. User clicks "Add Selected Faults"
9. All selected faults are instantly added to the experiment with their pre-configured settings
10. User can optionally fine-tune any setting before running the experiment

**For AI Agents:**

1. AI Agent receives a request: "Test the checkout service for network resilience"
2. AI Agent queries the system: "What Fault Studios are available for network testing?"
3. System returns: "Network Resilience Tests" studio with 5 configured faults
4. AI Agent creates an experiment and requests: "Add all enabled faults from 'Network Resilience Tests' studio"
5. System automatically:
   - Creates the experiment structure
   - Adds all 5 faults with their pre-configured settings
   - Targets the checkout service infrastructure
6. AI Agent triggers the experiment
7. Results are collected and analyzed automatically

### Visual Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        BEFORE INTEGRATION                           │
└─────────────────────────────────────────────────────────────────────┘

   User/AI Agent                    ChaosHub                Experiment
        │                              │                        │
        │── Browse Faults ────────────>│                        │
        │<── List of Raw Faults ───────│                        │
        │                              │                        │
        │── Select "pod-delete" ──────>│                        │
        │<── Fault Template ───────────│                        │
        │                              │                        │
        │── Configure Duration ────────────────────────────────>│
        │── Configure Target ──────────────────────────────────>│
        │── Configure Interval ────────────────────────────────>│
        │── Configure Weight ──────────────────────────────────>│
        │      (Repeat for each fault - time consuming!)        │
        │                              │                        │


┌─────────────────────────────────────────────────────────────────────┐
│                        AFTER INTEGRATION                            │
└─────────────────────────────────────────────────────────────────────┘

   User/AI Agent              Fault Studio               Experiment
        │                          │                         │
        │── Show My Studios ──────>│                         │
        │<── "Network Tests" ──────│                         │
        │    "DB Stress Pack"      │                         │
        │                          │                         │
        │── Select "Network Tests"─>│                         │
        │<── 5 Pre-configured ─────│                         │
        │    Faults Ready          │                         │
        │                          │                         │
        │── Add All to Experiment ────────────────────────────>│
        │                          │     (One Click!)         │
        │                          │                          │
        │<─────────────────── Experiment Ready ────────────────│
```

---

## 4. Success Criteria

This integration will be considered successful when:

1. **Time Savings**: Average experiment creation time reduced by 70% when using Fault Studios
2. **Adoption**: 50% of new experiments use faults from Fault Studios within 3 months of launch
3. **AI Agent Enablement**: AI Agents can successfully create and execute experiments using Fault Studios without manual intervention
4. **Error Reduction**: Configuration-related experiment failures reduced by 80%
5. **User Satisfaction**: Positive feedback from at least 80% of users surveyed about the new workflow

---

## 5. Key Stakeholder Questions Addressed

**Q: Does this replace ChaosHub?**  
A: No. ChaosHub remains the source of all fault definitions. Fault Studio simply allows users to save their preferred configurations from ChaosHub for quick reuse.

**Q: Can users still configure faults manually?**  
A: Yes. The ChaosHub tab remains available for users who want to configure faults from scratch or use one-off configurations.

**Q: Who creates the Fault Studios?**  
A: Any user with appropriate permissions can create Fault Studios. We recommend SRE teams or chaos engineering experts create standardized studios for their organizations.

**Q: How does this help with AI Agents?**  
A: AI Agents can reference Fault Studios by name instead of understanding complex fault parameters. This makes AI-driven chaos engineering practical and reliable.

**Q: What happens if someone modifies a Fault Studio?**  
A: Changes to a Fault Studio do not affect experiments that have already been created. Each experiment captures a snapshot of the fault configuration at creation time.

---

## 6. Timeline & Next Steps

| Phase | Description | Duration |
|-------|-------------|----------|
| Phase 1 | Backend API Development | 1 week |
| Phase 2 | Frontend UI Components | 1.5 weeks |
| Phase 3 | Integration & Testing | 1 week |
| Phase 4 | AI Agent Enablement | 0.5 weeks |
| Phase 5 | Documentation & Training | 0.5 weeks |
| **Total** | | **~5 weeks** |

---

## Appendix: Glossary

| Term | Definition |
|------|------------|
| **Fault** | A specific type of chaos injection (e.g., killing a pod, adding network delay) |
| **ChaosHub** | The central repository of available fault definitions |
| **Fault Studio** | A user-created collection of pre-configured faults |
| **Experiment** | A test that runs one or more faults against target infrastructure |
| **AI Agent** | An automated system that can create and execute experiments programmatically |
| **Tunables** | Configurable parameters of a fault (duration, target, intensity, etc.) |

---

*Document Version: 1.0*  
*Status: Ready for Stakeholder Review*
