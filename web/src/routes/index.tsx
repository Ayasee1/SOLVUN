import { useEffect, useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'

export const Route = createFileRoute('/')({ component: Home })

const SLIDES = [
  {
    src: '/images/campaign/solvun-portrait.jpg',
    srcSet:
      '/images/campaign/solvun-portrait.jpg 1x, /images/campaign/solvun-portrait@2x.jpg 2x, /images/campaign/solvun-portrait@3x.jpg 3x',
  },
  {
    src: '/images/campaign/DSC00437.JPG',
    srcSet:
      '/images/campaign/DSC00437.JPG 1x, /images/campaign/DSC00437@2x.jpg 2x',
  },
  {
    src: '/images/campaign/DSC00468.JPG',
    srcSet:
      '/images/campaign/DSC00468.JPG 1x, /images/campaign/DSC00468@2x.jpg 2x',
  },
  {
    src: '/images/campaign/DSC00469.JPG',
    srcSet:
      '/images/campaign/DSC00469.JPG 1x, /images/campaign/DSC00469@2x.jpg 2x',
  },
]

function Home() {
  const [active, setActive] = useState(0)

  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return
    }

    const timer = window.setInterval(() => {
      setActive((value) => (value + 1) % SLIDES.length)
    }, 5000)

    return () => window.clearInterval(timer)
  }, [])

  return (
    <main>
      <section className='grain relative h-[calc(100svh-4rem)] min-h-[480px] overflow-hidden bg-black text-white'>
        {SLIDES.map((slide, index) => (
          <img
            key={slide.src}
            src={slide.src}
            srcSet={slide.srcSet}
            sizes='100vw'
            alt=''
            fetchPriority={index === 0 ? 'high' : 'auto'}
            className={`absolute inset-0 h-full w-full object-cover object-center transition-opacity duration-1000 ${
              index === active ? 'opacity-100' : 'opacity-0'
            }`}
          />
        ))}

        <div className='absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-black/70' />

        <div className='animate-in fade-in slide-in-from-bottom-4 absolute inset-x-0 bottom-0 z-10 flex items-end justify-between p-4 pb-6 duration-700 sm:p-6'>
          <p className='flex items-center gap-2 text-[9px] tracking-[0.18em] text-white/70 uppercase'>
            <span className='bg-brand size-1.5 rounded-full' />
            After Hours — Vol. 01
            <span className='text-white/40 tabular-nums'>
              {String(active + 1).padStart(2, '0')} /{' '}
              {String(SLIDES.length).padStart(2, '0')}
            </span>
          </p>
          <Link
            to='/shop'
            className='text-[9px] tracking-[0.18em] text-white uppercase underline underline-offset-8 hover:no-underline'
          >
            Enter
          </Link>
        </div>
      </section>
    </main>
  )
}
