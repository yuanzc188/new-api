/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { ExternalLink, Copy, Video } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

interface VideoPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  url: string
}

export function VideoPreviewDialog(props: VideoPreviewDialogProps) {
  const { t } = useTranslation()
  const { url } = props
  const [hasError, setHasError] = useState(false)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHasError(false)
  }, [url])

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <Video className='h-5 w-5' />
          {t('Video Preview')}
        </>
      }
      contentClassName='sm:max-w-2xl'
      titleClassName='flex items-center gap-2'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {!url ? (
        <p className='text-muted-foreground py-4 text-center text-sm'>
          {t('None')}
        </p>
      ) : hasError ? (
        <div className='flex flex-wrap items-center gap-2 py-4'>
          <span className='text-destructive text-xs'>
            {t('Video playback failed')}
          </span>
          <Button
            variant='outline'
            size='sm'
            className='h-7 gap-1 text-xs'
            onClick={() => window.open(url, '_blank')}
          >
            <ExternalLink className='h-3 w-3' />
            {t('Open in new tab')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 gap-1 text-xs'
            onClick={() => {
              navigator.clipboard.writeText(url)
              toast.success(t('Copied'))
            }}
          >
            <Copy className='h-3 w-3' />
            {t('Copy Link')}
          </Button>
        </div>
      ) : (
        <div className='space-y-2'>
          <video
            src={url}
            controls
            preload='metadata'
            onError={() => setHasError(true)}
            className='bg-muted max-h-[60vh] w-full rounded-lg'
          />
          <div className='flex flex-wrap items-center justify-end gap-2'>
            <Button
              variant='outline'
              size='sm'
              className='h-7 gap-1 text-xs'
              onClick={() => window.open(url, '_blank')}
            >
              <ExternalLink className='h-3 w-3' />
              {t('Open in new tab')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              className='h-7 gap-1 text-xs'
              onClick={() => {
                navigator.clipboard.writeText(url)
                toast.success(t('Copied'))
              }}
            >
              <Copy className='h-3 w-3' />
              {t('Copy Link')}
            </Button>
          </div>
        </div>
      )}
    </Dialog>
  )
}
