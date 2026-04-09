/**
 * This module exports complex functions, including arrow functions,
 * closures, and IIFEs.
 */

import { SomeDependency } from './dep';

/**
 * A constant arrow function.
 */
export const myArrowFunction = (x: number, y: number): number => {
    return x + y;
};

export const myAsyncArrow = async (data: any) => {
    const result = await SomeDependency.fetch(data);
    return result;
};

export function outerFunction() {
    function innerFunction() {
        console.log("inner");
    }
    
    const anotherInner = () => {
        console.log("another inner");
        innerFunction();
    };
    
    return anotherInner;
}

class MyClass {
    /**
     * A class property that is an arrow function
     */
    public myMethod = () => {
        myArrowFunction(1, 2);
    }
}
