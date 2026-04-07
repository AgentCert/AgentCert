import React from 'react';
import { Button, ButtonVariation, Layout } from '@harnessio/uicore';
import { Dialog } from '@blueprintjs/core';
import css from './CertificateReportDialog.module.scss';

interface CertificateReportDialogProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
}

/* ================================================================
   Certificate Report Dialog — renders the static HTML template
   inside an iframe within a Blueprint Dialog popup.
   ================================================================ */
export default function CertificateReportDialog({
  isOpen,
  onClose,
  title = 'Agent Certification Report'
}: CertificateReportDialogProps): React.ReactElement {
  const handleDownload = (): void => {
    const link = document.createElement('a');
    link.href = '/sample_report.pdf';
    link.download = 'Agent_Certification_Report.pdf';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <Dialog
      isOpen={isOpen}
      canOutsideClickClose
      canEscapeKeyClose
      onClose={onClose}
      title={
        <Layout.Horizontal flex={{ alignItems: 'center', justifyContent: 'flex-start' }} spacing="medium">
          <span>{title}</span>
          <Button
            variation={ButtonVariation.SECONDARY}
            icon="import"
            text="Download PDF"
            onClick={handleDownload}
            small
          />
        </Layout.Horizontal>
      }
      className={css.certDialog}
    >
      <div className={css.iframeContainer}>
        <iframe
          src="/sample_report.html"
          title={title}
          className={css.reportIframe}
        />
      </div>
    </Dialog>
  );
}
