import { getTheme, getThemeName } from "../theme";

interface LogoProps {
    size?: number;
    animate?: boolean;
}

const S_PATH =
    "M2245 4820 c-131 -6 -300 -35 -410 -70 -33 -10 -76 -22 -95 -26 -136 -27 -512 -211 -673 -329 -268 -197 -402 -328 -594 -585 -79 -106 -215 -354 -269 -491 -12 -30 -31 -77 -42 -104 -22 -54 -37 -108 -83 -305 -45 -197 -63 -525 -39 -755 6 -61 14 -117 17 -125 3 -8 5 -27 4 -42 -1 -14 2 -29 8 -33 6 -3 11 -18 11 -33 0 -15 7 -49 15 -77 8 -27 23 -84 35 -125 11 -41 27 -89 35 -107 8 -17 15 -34 15 -38 0 -11 111 -257 121 -271 5 -6 21 -33 35 -60 14 -27 39 -68 55 -92 16 -24 29 -45 29 -48 0 -3 25 -39 55 -80 34 -45 60 -72 67 -68 7 4 8 3 4 -5 -11 -17 47 -85 208 -245 234 -232 494 -409 761 -520 44 -18 91 -37 105 -41 14 -4 39 -13 55 -21 71 -32 241 -73 425 -101 207 -33 627 -20 825 25 129 30 351 100 414 132 20 10 72 34 116 54 252 112 529 311 742 535 35 37 61 72 58 79 -3 7 -1 11 4 7 20 -12 212 252 298 409 43 79 109 216 118 247 4 13 13 38 21 54 23 51 55 147 71 215 8 36 19 73 24 83 12 23 39 199 39 258 0 25 5 50 11 56 15 15 14 391 -1 498 -6 44 -15 110 -21 147 -15 113 -104 399 -173 564 -115 274 -306 548 -532 763 -260 247 -559 431 -883 542 -142 49 -287 86 -388 99 -43 6 -98 15 -123 20 -62 12 -319 18 -475 10z m365 -841 c159 -25 334 -106 565 -259 217 -145 288 -185 402 -227 69 -25 116 -36 159 -37 l61 -1 -31 -50 c-76 -124 -179 -188 -313 -194 -150 -6 -284 57 -638 298 -131 90 -340 177 -481 202 -36 6 -105 9 -154 7 -75 -3 -97 -9 -156 -39 -56 -28 -75 -43 -99 -83 -26 -42 -30 -59 -29 -115 1 -182 162 -315 683 -567 377 -183 493 -243 613 -317 193 -120 316 -257 359 -398 29 -96 24 -292 -10 -389 -41 -115 -139 -286 -154 -270 -3 3 -1 32 4 65 11 74 3 159 -23 245 -29 93 -58 145 -125 224 -114 136 -279 242 -678 436 -462 225 -527 261 -651 356 -241 186 -295 432 -153 704 103 200 328 364 560 409 63 13 213 13 289 0z m-1110 -654 c-14 -60 -12 -172 5 -244 42 -174 150 -307 364 -445 72 -46 138 -80 651 -334 346 -171 480 -261 573 -383 85 -111 112 -205 105 -365 -8 -175 -71 -312 -207 -450 -126 -127 -268 -209 -426 -244 -114 -25 -218 -25 -342 -1 -170 33 -271 83 -558 274 -290 192 -407 247 -527 247 -30 0 -48 4 -48 12 0 21 61 106 102 144 60 54 139 86 224 92 144 9 236 -30 524 -220 222 -146 242 -158 371 -211 116 -48 186 -65 299 -73 354 -24 499 227 284 490 -96 118 -263 221 -730 451 -472 232 -585 302 -716 443 -165 179 -188 416 -63 674 31 63 115 192 121 185 2 -1 -1 -20 -6 -42z";

let idCounter = 0;

export function SpectraLogo({ size = 40, animate = false }: LogoProps) {
    const theme = getTheme(getThemeName());
    const id = ++idCounter;

    const animationStyle = animate
        ? `
            @keyframes spectra-spin-${id} {
                0%      { transform: rotate(0 deg); }
                20%     { transform: rotate(180deg); }
                30%     { transform: rotate(180deg) scale(1); }
                35%     { transform: rotate(180deg) scale(1.08); }
                40%     { transform: rotate(180deg) scale(1); }
                45%     { transform: rotate(180deg) scale(1.05); }
                50%     { transform: rotate(180deg) scale(1); }
                70%     { transform: rotate(360deg); }
                80%     { transform: rotate(360deg) scale(1); }
                85%     { transform: rotate(360deg) scale(1.08); }
                90%     { transform: rotate(360deg) scale(1); }
                95%     { transform: rotate(360deg) scale(1.05); }
                100%    { transform: rotate(360deg) scale(1); }
            }
        `
        : "";

    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 200 200"
            xmlns="http://www.w3.org/2000/svg"
            style={
                animate
                    ? {
                        animation: `spectra-spin-${id} 3s cubic-bezier(0.4, 0, 0.2, 1) infinite`,
                    }
                    : undefined
            }
        >
            {animate && <style>{animationStyle}</style>}
            <defs>
                <clipPath id={`spectra-circle-${id}`}>
                    <circle cx="100" cy="100" r="92" />
                </clipPath>
                <clipPath id={`spectra-s-clip-${id}`}>
                    <circle cx="100" cy="100" r="82" />
                </clipPath>
            </defs>

            {/* Two-tone circle */}
            <circle cx="100" cy="100" r="92" fill={theme.logoTop} />
            <rect
                x="0"
                y="100"
                width="200"
                height="100"
                fill={theme.logoBot}
                clipPath={`url(#spectra-circle-${id})`}
            />

            <circle
                cx="100"
                cy="100"
                r="92"
                fill="none"
                stroke="rgba(255,255,255,0.08)"
                strokeWidth="1.5"
            />

            <g clipPath={`url(#spectra-s-clip-${id})`}>
                <g
                    transform="translate(100,100) scale(0.034,-0.034) translate(-2435,-2420)"
                    fill="white"
                >
                    <path d={S_PATH} />
                </g>
            </g>
        </svg>
    );
}