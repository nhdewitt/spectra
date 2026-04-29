import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import {
    StatBlock,
    DetailRow,
    ViewToggle,
    LoadingText,
    MetricSelector,
    InstructionBlock,
} from '../components/ui'

describe('StatBlock', () => {
    it('renders label and value', () => {
        render(<StatBlock label="CPU" value="45.2%" />)
        expect(screen.getByText('CPU')).toBeInTheDocument()
        expect(screen.getByText('45.2%')).toBeInTheDocument()
    })

    it('renders dash for null value', () => {
        render(<StatBlock label="Uptime" value={null} />)
        expect(screen.getByText('—')).toBeInTheDocument()
    })

    it('renders unit when provided', () => {
        render(<StatBlock label="RAM" value="16" unit="GB" />)
        expect(screen.getByText('16')).toBeInTheDocument()
        expect(screen.getByText('GB')).toBeInTheDocument()
    })

    it('does not render unit when omitted', () => {
        render(<StatBlock label="Users" value="3" />)
        expect(screen.getByText('3')).toBeInTheDocument()
        expect(screen.queryByText('GB')).toBeNull()
    })
})

describe('DetailRow', () => {
    it('renders label and value', () => {
        render(<DetailRow label="OS" value="linux" />)
        expect(screen.getByText('OS')).toBeInTheDocument()
        expect(screen.getByText('linux')).toBeInTheDocument()
    })

    it('renders dash for null value', () => {
        render(<DetailRow label="Arch" value={null} />)
        expect(screen.getByText('—')).toBeInTheDocument()
    })

    it('renders dash for undefined value', () => {
        render(<DetailRow label="Arch" value={undefined} />)
        expect(screen.getByText('—')).toBeInTheDocument()
    })

    it('renders numeric values', () => {
        render(<DetailRow label="Cores" value={8} />)
        expect(screen.getByText('8')).toBeInTheDocument()
    })
})

describe('ViewToggle', () => {
    it('renders tiles and list buttons', () => {
        render(<ViewToggle mode="tiles" onChange={() => {}} />)
        expect(screen.getByText(/tiles/i)).toBeInTheDocument()
        expect(screen.getByText(/list/i)).toBeInTheDocument()
    })

    it('calls onChange with selected mode', () => {
        const onChange = vi.fn()
        render(<ViewToggle mode="tiles" onChange={onChange} />)
        fireEvent.click(screen.getByText(/list/i))
        expect(onChange).toHaveBeenCalledWith('list')
    })
})

describe('LoadingText', () => {
    it('renders loading text', () => {
        vi.useFakeTimers()
        render(<LoadingText />)
        expect(screen.getByText('Loading')).toBeInTheDocument()

        act(() => { vi.advanceTimersByTime(400) })
        expect(screen.getByText('Loading.')).toBeInTheDocument()

        act(() => { vi.advanceTimersByTime(400) })
        expect(screen.getByText('Loading..')).toBeInTheDocument()

        act(() => { vi.advanceTimersByTime(400) })
        expect(screen.getByText('Loading...')).toBeInTheDocument()

        act(() => { vi.advanceTimersByTime(400) })
        expect(screen.getByText('Loading')).toBeInTheDocument()

        vi.useRealTimers()
    })
})

describe('MetricSelector', () => {
    it('renders nothing with single option', () => {
        const { container } = render(<MetricSelector label="Device" options={['sda']} value="sda" onChange={() => {}} />)
        expect(container.innerHTML).toBe('')
    })

    it('renders select with multiple options', () => {
        render(<MetricSelector label="Device" options={['sda', 'sdb']} value="sda" onChange={() => {}} />)
        expect(screen.getByText('Device:')).toBeInTheDocument()
        expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    it('calls onChange on selection', () => {
        const onChange = vi.fn()
        render(<MetricSelector label="Device" options={['sda', 'sdb']} value="sda" onChange={onChange} />)
        fireEvent.change(screen.getByRole('combobox'), { target: { value: 'sdb' } })
        expect(onChange).toHaveBeenCalledWith('sdb')
    })
})

describe('InstructionBlock', () => {
  const steps = `1. Install the binary
sudo install -m 0755 spectra-agent /usr/local/bin/spectra-agent

2. Start the service
sudo systemctl enable --now spectra-agent`

  it('renders title and steps', () => {
    render(<InstructionBlock title="Install" steps={steps} onClose={() => {}} />)
    expect(screen.getByText('Install')).toBeInTheDocument()
    expect(screen.getByText(/Install the binary/)).toBeInTheDocument()
  })

  it('calls onClose when × is clicked', () => {
    const onClose = vi.fn()
    render(<InstructionBlock title="Install" steps={steps} onClose={onClose} />)
    fireEvent.click(screen.getByText('×'))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls onClose when clicking backdrop', () => {
    const onClose = vi.fn()
    const { container } = render(
      <InstructionBlock title="Install" steps={steps} onClose={onClose} />
    )
    // The backdrop is the outermost fixed-position div
    const backdrop = container.firstChild as HTMLElement
    fireEvent.click(backdrop)
    expect(onClose).toHaveBeenCalled()
  })

  it('copies commands without numbered headers', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })

    render(<InstructionBlock title="Install" steps={steps} onClose={() => {}} />)
    fireEvent.click(screen.getByText('Copy commands'))

    expect(writeText).toHaveBeenCalledWith(
      'sudo install -m 0755 spectra-agent /usr/local/bin/spectra-agent\nsudo systemctl enable --now spectra-agent'
    )
  })

  it('renders footer when provided', () => {
    render(
      <InstructionBlock
        title="Install"
        steps={steps}
        onClose={() => {}}
        footer={<div>Download binary</div>}
      />
    )
    expect(screen.getByText('Download binary')).toBeInTheDocument()
  })
})