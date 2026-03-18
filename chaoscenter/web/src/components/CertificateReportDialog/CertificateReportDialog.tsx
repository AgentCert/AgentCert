import React from 'react';
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
  return (
    <Dialog
      isOpen={isOpen}
      canOutsideClickClose
      canEscapeKeyClose
      onClose={onClose}
      title={title}
      className={css.certDialog}
    >
      <div className={css.iframeContainer}>
        {/* <iframe
          src="/sample_report1.html"
          title={title}
          className={css.reportIframe}
        /> */}
        <iframe
          src="/sample_report.html"
          title={title}
          className={css.reportIframe}
        />
        {/* <iframe
          src="/certificate_report.html"
          title={title}
          className={css.reportIframe}
        /> */}
      </div>
    </Dialog>
  );
}
