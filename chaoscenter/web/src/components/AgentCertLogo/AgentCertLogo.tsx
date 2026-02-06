import React from 'react';
import agentCertLogo from './agentcert-logo.png';
import css from './AgentCertLogo.module.scss';

interface AgentCertLogoProps {
  size?: number;
  className?: string;
}

export default function AgentCertLogo({ size = 30, className }: AgentCertLogoProps): React.ReactElement {
  return <img src={agentCertLogo} alt="AgentCert" width={size} height={size} className={className || css.logo} />;
}
