import { Fragment } from 'react';
import { LEGAL_LINKS } from '../lib/legal';

interface LegalLinksProps {
  className?: string;
  linkClassName?: string;
  inline?: boolean;
}

export default function LegalLinks({
  className = '',
  linkClassName = '',
  inline = false,
}: Readonly<LegalLinksProps>) {
  const classes = ['legal-links', inline ? 'legal-links--inline' : '', className].filter(Boolean).join(' ');

  return (
    <div className={classes}>
      {LEGAL_LINKS.map((link, index) => (
        <Fragment key={link.href}>
          <a href={link.href} target="_blank" rel="noopener" className={linkClassName || undefined}>
            {link.label}
          </a>
          {inline && index < LEGAL_LINKS.length - 1 && (
            <span aria-hidden="true" className="legal-links__separator">•</span>
          )}
        </Fragment>
      ))}
    </div>
  );
}
