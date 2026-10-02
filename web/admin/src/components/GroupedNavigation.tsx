import { Box, Collapse, Group, NavLink, Stack, Text, UnstyledButton } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconChevronDown } from '@tabler/icons-react'
import { useState, type ComponentType } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import classes from './GroupedNavigation.module.css'

interface Item { to: string; labelKey: string; secondary?: boolean }
interface Section { id: string; labelKey: string; items: Item[] }
export function GroupedNavigation({ groups, icons, activeGroup, onNavigate }: { groups: Section[]; icons: Record<string, ComponentType<{ size?: number; stroke?: number }>>; activeGroup: string; onNavigate: () => void }) {
  const { t } = useTranslation()
  const location = useLocation()
  const mobile = useMediaQuery('(max-width: 47.99em)')
  // Expansion belongs to the user's navigation session, not a single route.
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [more, setMore] = useState<Record<string, { open: boolean; route: string }>>({})
  const params = new URLSearchParams(location.search)
  const activeItem = groups.flatMap(group => group.items).find(item => {
    const [path, search] = item.to.split('?')
    return path === location.pathname && !!search && [...new URLSearchParams(search)].every(([key, value]) => params.get(key) === value)
  }) ?? groups.flatMap(group => group.items).find(item => !item.to.includes('?') && (item.to === location.pathname || (item.to !== '/' && location.pathname.startsWith(`${item.to}/`))))
  const links = (items: Item[], fallback?: ComponentType<{ size?: number; stroke?: number }>) => items.map(item => { const Icon = icons[item.to] ?? icons[item.to.split('?')[0]] ?? fallback; return <NavLink
    key={item.to} component={Link} to={item.to} label={t(item.labelKey)} active={activeItem === item}
    aria-current={activeItem === item ? 'page' : undefined} onClick={onNavigate}
    leftSection={Icon && <Icon size={18} stroke={1.6} />} className={classes.link}
  /> })
  return <Box component="nav" aria-label={t('workspace.navigation')}><Stack gap={12}>{groups.map(group => {
    const Icon = icons[group.id]
    if (group.items.length === 1) return <Box key={group.id}>{links(group.items, Icon)}</Box>
    const opened = expanded[group.id] ?? activeGroup === group.id
    const secondary = group.items.filter(item => item.secondary)
    const activeSecondary = secondary.includes(activeItem!)
    const choice = more[group.id]
    const moreOpened = activeSecondary && choice?.route !== location.key ? true : choice?.open ?? activeSecondary
    const moreButton = secondary.length > 0 && <UnstyledButton className={classes.moreToggle} aria-label={t('workspace.moreIn', { section: t(group.labelKey) })} aria-expanded={moreOpened} aria-controls={`nav-more-${group.id}`} onClick={() => setMore(previous => ({ ...previous, [group.id]: { open: !moreOpened, route: location.key } }))}>
      <Group gap={4} wrap="nowrap"><Text component="span" size="xs">{t('workspace.more')}</Text><IconChevronDown size={13} style={{ flexShrink: 0, transform: moreOpened ? 'rotate(180deg)' : undefined }} /></Group>
    </UnstyledButton>
    const heading = <Group justify="space-between" wrap="nowrap" gap="xs"><Text component="span" className={classes.heading}>{t(group.labelKey)}</Text>{mobile ? <IconChevronDown size={15} style={{ transform: opened ? 'rotate(180deg)' : undefined }} /> : moreButton}</Group>
    return <Box key={group.id}>
      {mobile ? <UnstyledButton className={classes.sectionToggle} aria-expanded={opened} aria-controls={`nav-${group.id}`} onClick={() => setExpanded(previous => ({ ...previous, [group.id]: !opened }))}>{heading}</UnstyledButton> : <Box px="sm" mb={4}>{heading}</Box>}
      <Collapse in={!mobile || opened}><Stack id={`nav-${group.id}`} gap={2}>
        {links(group.items.filter(item => !item.secondary))}
        {secondary.length > 0 && <>
          {mobile && moreButton}
          <Collapse in={moreOpened}><Stack id={`nav-more-${group.id}`} gap={2}>{links(secondary)}</Stack></Collapse>
        </>}
      </Stack></Collapse>
    </Box>
  })}</Stack></Box>
}
