import { MeteorMathJax } from 'meteor/mrt:mathjax';

// The MathJax CDN has shut down, <https://www.mathjax.org/cdn-shutting-down/>.
// Configure `mrt:mathjax` to use the new CDN as recommended in the shutdown
// notice with the config param from the `mrt:mathjax` source,
// <https://github.com/apendua/meteor-mathjax/tree/master#configuration>.
MeteorMathJax.sourceUrl = 'https://cdnjs.cloudflare.com/ajax/libs/mathjax/2.7.1/MathJax.js?config=TeX-AMS-MML_HTMLorMML';
