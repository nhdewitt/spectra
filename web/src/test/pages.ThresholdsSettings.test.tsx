import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ThresholdsSettings } from '../pages/ThresholdsSettings'
import type { Thresholds } from '../types'
import { DEFAULT_THRESHOLDS } from '../types'

vi.mock('../api', () => ({
    api: { thresholds: vi.fn(), updateThresholds: vi.fn() },
}))

import { api } from '../api'
const mockThresholds = api.thresholds as ReturnType<typeof vi.fn>
const mockUpdateThresholds = api.updateThresholds as ReturnType<typeof vi.fn>

beforeEach(() => {
    mockThresholds.mockReset().mockResolvedValue(DEFAULT_THRESHOLDS)
    mockUpdateThresholds.mockReset()
})

describe('ThresholdsSettings', () => {
    it('shows a loading spinner, then the loaded values', async () => {
        const custom: Thresholds = { ...DEFAULT_THRESHOLDS, cpu_warn: 75 }
        mockThresholds.mockResolvedValue(custom)

        const { container } = render(<ThresholdsSettings />)
        expect(container.querySelector('svg')).toBeInTheDocument()

        await waitFor(() => expect(screen.getByDisplayValue('75')).toBeInTheDocument())
    })

    it('saves the edited values', async () => {
        mockUpdateThresholds.mockResolvedValue({ ...DEFAULT_THRESHOLDS, cpu_warn: 70 })

        render(<ThresholdsSettings />)
        await waitFor(() => expect(screen.getAllByDisplayValue('80').length).toBeGreaterThan(0))

        // cpu_warn and mem_warn are both 80 by default - the first (CPU row) is index 0.
        fireEvent.change(screen.getAllByDisplayValue('80')[0]!, { target: { value: '70' } })
        fireEvent.click(screen.getByText('Save thresholds'))

        await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument())
        expect(mockUpdateThresholds).toHaveBeenCalledWith(
            expect.objectContaining({ cpu_warn: 70 })
        )
    })

    it('clears the "Saved" indicator as soon as a field is edited again', async () => {
        mockUpdateThresholds.mockResolvedValue(DEFAULT_THRESHOLDS)

        render(<ThresholdsSettings />)
        await waitFor(() => expect(screen.getByText('Save thresholds')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Save thresholds'))
        await waitFor(() => expect(screen.getByText('Saved')).toBeInTheDocument())

        fireEvent.change(screen.getByDisplayValue('120'), { target: { value: '90' } })
        expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    })

    it('shows a saving state while the save request is in flight', async () => {
        let resolveSave!: (t: Thresholds) => void
        mockUpdateThresholds.mockReturnValue(new Promise<Thresholds>((res) => { resolveSave = res }))

        render(<ThresholdsSettings />)
        await waitFor(() => expect(screen.getByText('Save thresholds')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Save thresholds'))
        expect(screen.getByText('Saving...')).toBeInTheDocument()
        expect(screen.getByText('Saving...')).toBeDisabled()

        resolveSave(DEFAULT_THRESHOLDS)
        await waitFor(() => expect(screen.getByText('Save thresholds')).toBeInTheDocument())
    })

    it('shows an error message when saving fails', async () => {
        mockUpdateThresholds.mockRejectedValue(new Error('forbidden'))

        render(<ThresholdsSettings />)
        await waitFor(() => expect(screen.getByText('Save thresholds')).toBeInTheDocument())

        fireEvent.click(screen.getByText('Save thresholds'))
        await waitFor(() => expect(screen.getByText('forbidden')).toBeInTheDocument())
        expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    })
})