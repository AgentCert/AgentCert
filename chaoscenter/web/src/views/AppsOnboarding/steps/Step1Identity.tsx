import React from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData } from '../types';
import { DOMAINS } from '../types';
import css from '../AppsOnboarding.module.scss';

interface Step1Props {
  data: ContributionFormData;
  onNext: (patch: Partial<ContributionFormData>) => void;
}

export default function Step1Identity({ data, onNext }: Step1Props): React.ReactElement {
  const [values, setValues] = React.useState({
    name: data.name,
    displayName: data.displayName,
    domain: data.domain,
    shortDescription: data.shortDescription,
    longDescription: data.longDescription,
    maintainerName: data.maintainerName,
    maintainerEmail: data.maintainerEmail,
  });
  const [errors, setErrors] = React.useState<Record<string, string>>({});

  const validate = (): boolean => {
    const errs: Record<string, string> = {};
    if (!/^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(values.name)) errs.name = 'Kebab-case only (a-z, 0-9, hyphens). Must not start/end with hyphen.';
    if (values.name.length > 63) errs.name = 'Max 63 characters';
    if (!values.displayName) errs.displayName = 'Required';
    if (!values.domain) errs.domain = 'Select a domain';
    if (values.shortDescription.length < 10) errs.shortDescription = 'Min 10 characters';
    if (values.shortDescription.length > 120) errs.shortDescription = 'Max 120 characters';
    if (!values.longDescription) errs.longDescription = 'Required';
    if (!values.maintainerName) errs.maintainerName = 'Required';
    if (!/^[^@]+@[^@]+\.[^@]+$/.test(values.maintainerEmail)) errs.maintainerEmail = 'Invalid email';
    setErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleNext = (): void => {
    if (validate()) onNext(values);
  };

  const field = (key: keyof typeof values, label: string, placeholder?: string, textarea?: boolean): React.ReactElement => (
    <Layout.Vertical spacing="xsmall" key={key}>
      <Text font={{ variation: FontVariation.FORM_LABEL }} color={Color.GREY_700}>{label}</Text>
      {textarea ? (
        <textarea
          className={css.input}
          value={values[key] as string}
          onChange={e => setValues(v => ({ ...v, [key]: e.target.value }))}
          placeholder={placeholder}
          rows={4}
          style={{ resize: 'vertical', fontFamily: 'inherit' }}
        />
      ) : (
        <input
          className={`${css.input} ${errors[key] ? css.inputError : ''}`}
          value={values[key] as string}
          onChange={e => setValues(v => ({ ...v, [key]: e.target.value }))}
          placeholder={placeholder}
        />
      )}
      {errors[key] && <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>{errors[key]}</Text>}
    </Layout.Vertical>
  );

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 1 of 6 — App Identity
      </Text>
      <Layout.Vertical spacing="large">
        {field('name', 'App Name (kebab-case) *', 'my-app-name')}
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
          This becomes the stable ID. Cannot change after any experiment references this app.
        </Text>
        {field('displayName', 'Display Name *', 'My App Name')}
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.FORM_LABEL }} color={Color.GREY_700}>Domain *</Text>
          <select
            className={`${css.input} ${errors.domain ? css.inputError : ''}`}
            value={values.domain}
            onChange={e => setValues(v => ({ ...v, domain: e.target.value }))}
          >
            <option value="">Select a domain...</option>
            {DOMAINS.map(d => <option key={d.id} value={d.id}>{d.displayName}</option>)}
          </select>
          {errors.domain && <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>{errors.domain}</Text>}
        </Layout.Vertical>
        <Layout.Vertical spacing="xsmall">
          {field('shortDescription', 'Short Description * (≤120 chars)', 'A one-line description for the catalog card')}
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>{values.shortDescription.length}/120</Text>
        </Layout.Vertical>
        {field('longDescription', 'Full Description *', 'Describe your application...', true)}
        <Layout.Horizontal spacing="medium">
          <Layout.Vertical className={css.halfWidth}>
            {field('maintainerName', 'Maintainer Name *')}
          </Layout.Vertical>
          <Layout.Vertical className={css.halfWidth}>
            {field('maintainerEmail', 'Maintainer Email *', 'you@example.com')}
          </Layout.Vertical>
        </Layout.Horizontal>
        <Layout.Horizontal flex={{ justifyContent: 'flex-end' }}>
          <Button variation={ButtonVariation.PRIMARY} text="Next: Installation →" onClick={handleNext} />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
